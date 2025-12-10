package gotrader

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"github.com/redis/rueidis"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"log"
	"math"
	"os"
	"time"
)

var (
	ErrMetricNotFound = errors.New("metric not founc")

	MCandleOpen   = NewMetricWithDefaultViews("candle_open")
	MCandleHigh   = NewMetricWithDefaultViews("candle_high")
	MCandleClose  = NewMetricWithDefaultViews("candle_close")
	MCandleLow    = NewMetricWithDefaultViews("candle_low")
	MCandleVolume = NewMetricWithDefaultViews("candle_volume")
	MTradesBuy    = NewMetricWithDefaultViews("trades_buy")
	MTradesSell   = NewMetricWithDefaultViews("trades_sell")
	MCash         = NewMetricWithDefaultViews("cash")
	MStartingCash = NewMetricWithDefaultViews("cash_initial")
	MPosition     = NewMetricWithDefaultViews("position")

	KeySymbol, _ = tag.NewKey("symbol")

	DisableMetrics = false
)

func GetNewContextFromCandle(c Candle) context.Context {
	// The tags are hdden in OpenCensus.
	// We need to have access to the candle, so we duplicate it.
	ctx := context.WithValue(context.Background(), candleCtxKey, c)
	ctx, err := tag.New(ctx,
		tag.Insert(KeySymbol, string(c.Symbol)),
	)
	if err != nil {
		panic(err) // This should never happen, really
	}

	return ctx
}

var registeredViews []*view.View

// RegisterViews register the Opencensus views and enable the internal TimeSeries collection
func RegisterViews(views ...*view.View) {

	if err := view.Register(views...); err != nil {
		log.Fatalf("Failed to register views: %v", err)
	}

	registeredViews = append(registeredViews, views...)
}

type Metric struct {
	Name    string
	measure *stats.Float64Measure
}

var localDb = MemorySignals{}

type ctxKey struct{}

var candleCtxKey = ctxKey{}

func (m *Metric) RecordBatch(candles []Candle, value float64) {
	if DisableMetrics {
		return
	}
	for _, c := range candles {
		localDb.Append(c, m.Name, value)
	}
}

func (m *Metric) Record(ctx context.Context, value float64) {
	if DisableMetrics {
		return
	}

	stats.Record(ctx, m.measure.M(value))

	c := ctx.Value(candleCtxKey)
	if c == nil {
		return
	}
	localDb.Append(c.(Candle), m.Name, value)
}

// Get the i-th metric. Metrics are saved in their chronological order. m.Get(0) returns the last value recorder by the metric m.
// m.Get(-3) return the value inserted 3 "step" ago. A step is defined as a full eval() cycle.
func (m *Metric) Get(ctx context.Context, step int) (float64, error) {
	if DisableMetrics {
		return 0, nil
	}

	i := int(math.Abs(float64(step)))
	c := ctx.Value(candleCtxKey)
	return localDb.Get(c.(Candle), m.Name, i)
}

func NewMetricWithDefaultViews(name string) *Metric {
	m := stats.Float64(name, "", stats.UnitDimensionless)
	// Warning: the RedisExporter support only view.LastValue()as Aggregation.
	// If you change it here, it will panic
	v := &view.View{Measure: m, Aggregation: view.LastValue(), TagKeys: []tag.Key{KeySymbol}}

	err := view.Register(v)
	if err != nil {
		panic(err)
	}
	return &Metric{
		Name:    m.Name(),
		measure: m,
	}
}

type TimeSerie struct {
	X []time.Time `json:"x"`
	Y []float64   `json:"y"`
}

// Append an element to the end of this ts
func (ts *TimeSerie) Append(candle Candle, value float64) {
	if DisableMetrics {
		return
	}

	ts.X = append(ts.X, candle.Time)
	ts.Y = append(ts.Y, value)
}

type MemorySignals struct {
	Metrics map[string]*TimeSerie
}

// Append a metric to a given signal.
func (s *MemorySignals) Append(candle Candle, name string, value float64) {
	if DisableMetrics {
		return
	}

	if s.Metrics == nil {
		s.Metrics = map[string]*TimeSerie{}
	}

	key := string(candle.Symbol) + "." + name
	_, exists := s.Metrics[key]
	if !exists {
		s.Metrics[key] = &TimeSerie{
			X: []time.Time{},
			Y: []float64{},
		}
	}

	s.Metrics[key].Append(candle, value)

}

func (s *MemorySignals) Get(candle Candle, name string, i int) (float64, error) {
	if DisableMetrics {
		return 0, nil
	}

	ts, found := s.Metrics[string(candle.Symbol)+"."+name]
	if !found {
		return 0, ErrMetricNotFound
	}

	index := len(ts.Y) - 1 - i
	if index < 0 {
		return 0, errors.New("index too old")
	}

	return ts.Y[index], nil
}

type RedisExporter struct {
	// MetricNameGenerator MUST return a string formatted as `gotrader.<symbol>.<metric>`
	MetricNameGenerator func(vd *view.Data, row *view.Row) string
	redis               rueidis.Client
}

func NewRedisExporter(redisHostPort string) (*RedisExporter, error) {

	client, err := rueidis.NewClient(rueidis.ClientOption{InitAddress: []string{redisHostPort}})
	if err != nil {
		panic(err)
	}

	pingCmd := client.B().Ping().Build()
	resp := client.Do(context.Background(), pingCmd)

	daje, err := resp.AsBytes()
	if err != nil {
		panic(err)
	}
	println(fmt.Sprintf("PING? %s", string(daje)))

	set := client.B().Set().Key("dio").Value("cane").Build()
	resp2 := client.Do(context.Background(), set)

	daje2, err := resp2.AsBytes()
	if err != nil {
		panic(err)
	}
	println(fmt.Sprintf("PING? %s", string(daje2)))

	return &RedisExporter{
		MetricNameGenerator: DefaultViewToName,
		redis:               client,
	}, nil
}

func (exp RedisExporter) ExportView(vd *view.Data) {

	for _, row := range vd.Rows {
		metricName := exp.MetricNameGenerator(vd, row)
		value, supportedAggregation := row.Data.(*view.LastValueData)
		if !supportedAggregation {
			panic("RedisExporter supports only view.LastValueData as Aggregation")
		}

		exp.redis.Do(context.Background(), exp.redis.B().TsAdd().Key(metricName).Timestamp(vd.End.UnixMilli()).Value(value.Value).Build())
	}

}

// FlushBuffer saves the commands in a .redis file, that can be imported later
func (exp RedisExporter) FlushBuffer(path string) {
	if DisableMetrics {
		return
	}

	metrics := localDb.Metrics
	if metrics == nil || len(metrics) == 0 {
		panic("FlushBuffer() works only in backtesting with Memorysignals")
	}

	fname := fmt.Sprintf("%s.redis", time.Now().Format("20060102150405"))
	archive, err := os.Create(path + string(os.PathSeparator) + fname + ".zip")
	if err != nil {
		panic(err)
	}
	defer func() { _ = archive.Close() }()

	zipWriter := zip.NewWriter(archive)
	if err != nil {
		panic(err)
	}
	defer func() {
		_ = zipWriter.Flush()
		_ = zipWriter.Close()
	}()
	writer, err := zipWriter.Create(fname)

	for mKey, mValue := range metrics {
		for i := range mValue.X {
			chunkSize := 4096 * 16
			// _, _ = writer.Write([]byte(fmt.Sprintf("TS.ADD gotrader.%s %v %v ENCODING COMPRESSED CHUNK_SIZE %v DUPLICATE_POLICY LAST \n", mKey, mValue.X[i].UnixMilli(), mValue.Y[i], chunkSize)))
			_, _ = writer.Write([]byte(fmt.Sprintf("TS.ADD gotrader.%s %v %v ENCODING COMPRESSED CHUNK_SIZE %v DUPLICATE_POLICY LAST \n", mKey, mValue.X[i].UnixMilli(), mValue.Y[i], chunkSize)))
		}
	}

	localDb.Metrics = map[string]*TimeSerie{}

}

func (exp RedisExporter) Flush() {
	if DisableMetrics {
		return
	}

	metrics := localDb.Metrics
	if metrics == nil || len(metrics) == 0 {
		panic("flush() works only in backtesting with Memorysignals")
	}

	// slog.Info("REDIS FLUSH ALL")
	// fres := exp.redis.Do(context.Background(), exp.redis.B().Flushall().Sync().Build())
	// if fres.Error() != nil {
	// 	panic(fres.Error())
	// }
	// for mKey, mValue := range metrics {
	// 	createCmd := exp.redis.B().TsCreate().
	// 		Key(mKey).
	// 		DuplicatePolicyLast().  // sovrascrive con l'ultimo valore
	// 		Build()
	//
	// 	exp.redis.Do(context.Background(), createCmd)
	// }

	var cmds []rueidis.Completed
	// cache := map[string]bool{}

	for mKey, mValue := range metrics {
		for i := range mValue.X {
			// cache[mKey] = true

			// println(fmt.Sprintf("%s => %v", fmt.Sprintf("gotrader.%s", mKey)), mValue.Y[i])
			// fres := exp.redis.Do(context.Background(), exp.redis.B().TsAdd().Key(fmt.Sprintf("gotrader.%s", mKey)).Timestamp(mValue.X[i].UnixMilli()).Value(mValue.Y[i]).Build())
			// if fres.Error() != nil {
			// 	panic(fres.Error())
			// }

			c := exp.redis.B().
				TsAdd().
				Key(fmt.Sprintf("gotrader.%s", mKey)).
				Timestamp(mValue.X[i].UnixMilli()).
				Value(mValue.Y[i]).
				OnDuplicateLast().
				Build()
			cmds = append(cmds, c)
		}

	}

	// for k, _ := range cache {
	// 	println(k)
	// }
	// time.Sleep(50000 * time.Second)

	results := exp.redis.DoMulti(context.Background(), cmds...)

	for i, r := range results {
		if r.Error() != nil {
			println(i)
			println(r.Error().Error())
		}
	}

	// Flush() menas that all metrics are sent and
	localDb.Metrics = map[string]*TimeSerie{}
}

func (exp RedisExporter) Set(key string, value float64) {
	c := exp.redis.B().
		Set().Key(fmt.Sprintf("gotrader.%s", key)).
		Value(fmt.Sprintf("%4f", value)).
		Build()
	exp.redis.Do(context.Background(), c)
}

func DefaultViewToName(vd *view.Data, row *view.Row) string {
	tagSymbol := "__missing__"
	for _, t := range row.Tags {
		if t.Key.Name() == "symbol" {
			tagSymbol = t.Value
		}
	}

	return fmt.Sprintf("gotrader.%s.%s", tagSymbol, vd.View.Name)
}
