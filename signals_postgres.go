package gotrader

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opencensus.io/stats/view"
)

type PostgresExporter struct {
	pool *pgxpool.Pool
}

func NewPostgresExporter(connStr string) (*PostgresExporter, error) {
	pool, err := pgxpool.New(context.Background(), connStr)
	if err != nil {
		return nil, fmt.Errorf("connecting to postgres: %w", err)
	}

	_, err = pool.Exec(context.Background(), `
		CREATE TABLE IF NOT EXISTS gotrader_metrics (
			runid  INTEGER          NOT NULL,
			time   TIMESTAMPTZ      NOT NULL,
			symbol TEXT             NOT NULL,
			metric TEXT             NOT NULL,
			value  DOUBLE PRECISION NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_gotrader_metrics_time   ON gotrader_metrics (time);
		CREATE INDEX IF NOT EXISTS idx_gotrader_metrics_lookup ON gotrader_metrics (runid, symbol, metric, time);
	`)
	if err != nil {
		return nil, fmt.Errorf("creating table: %w", err)
	}

	return &PostgresExporter{pool: pool}, nil
}

// ExportView is a no-op: in backtest mode all metrics are captured in localDb via Metric.Record.
func (exp *PostgresExporter) ExportView(_ *view.Data) {}

// Flush writes all accumulated metrics to Postgres in a single COPY operation.
// Every call to Flush gets a new runid, incremented from the current max.
func (exp *PostgresExporter) Flush() {
	if DisableMetrics {
		return
	}

	metrics := localDb.Metrics
	if len(metrics) == 0 {
		return
	}

	ctx := context.Background()

	var runid int
	err := exp.pool.QueryRow(ctx, `SELECT COALESCE(MAX(runid), 0) + 1 FROM gotrader_metrics`).Scan(&runid)
	if err != nil {
		panic(fmt.Errorf("getting next runid: %w", err))
	}

	total := 0
	for _, ts := range metrics {
		total += len(ts.X)
	}

	rows := make([][]any, 0, total)
	for key, ts := range metrics {
		parts := strings.SplitN(key, ".", 2)
		symbol, metric := parts[0], parts[1]
		for i := range ts.X {
			rows = append(rows, []any{runid, ts.X[i], symbol, metric, ts.Y[i]})
		}
	}

	_, err = exp.pool.CopyFrom(
		ctx,
		pgx.Identifier{"gotrader_metrics"},
		[]string{"runid", "time", "symbol", "metric", "value"},
		pgx.CopyFromRows(rows),
	)
	if err != nil {
		panic(fmt.Errorf("copying metrics to postgres: %w", err))
	}

	localDb.Metrics = map[string]*TimeSerie{}
}
