package gotrader

import (
	"context"
	"errors"
	"fmt"
	"github.com/redis/rueidis"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"log"
	"math"
	"time"
)

var (
	ErrMetricNotFound = errors.New("metric not founc")

	MCandleOpen  = NewMetricWithDefaultViews("candle_open")
	MCandleHigh  = NewMetricWithDefaultViews("candle_high")
	MCandleClose = NewMetricWithDefaultViews("candle_close")
	MCandleLow   = NewMetricWithDefaultViews("candle_low")
	MTradesBuy   = NewMetricWithDefaultViews("trades_buy")
	MTradesSell  = NewMetricWithDefaultViews("trades_sell")
	MCash        = NewMetricWithDefaultViews("cash")
	MPosition    = NewMetricWithDefaultViews("position")

	KeySymbol, _ = tag.NewKey("symbol")
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

func (m *Metric) Record(ctx context.Context, value float64) {
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
	ts.X = append(ts.X, candle.Time)
	ts.Y = append(ts.Y, value)
}

type MemorySignals struct {
	Metrics map[string]*TimeSerie
}

// Append a metric to a given signal.
func (s *MemorySignals) Append(candle Candle, name string, value float64) {

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
		println(fmt.Sprintf("TS.ADD %s %v %v", metricName, vd.End.UnixMilli(), value.Value))

	}

}

func (exp RedisExporter) Flush() {
	metrics := localDb.Metrics
	if metrics == nil || len(metrics) == 0 {
		Stderr.Panicf("flush() works only in backtesting with Memorysignals")
	}

	var cmds []rueidis.Completed

	for mKey, mValue := range metrics {

		// mKey := <symbol>.<metric>
		for i := range mValue.X {
			// println(fmt.Sprintf("TS.ADD gotrader.%s %v %v", mKey, mValue.X[i].UnixMilli(), mValue.Y[i]))
			c := exp.redis.B().TsAdd().Key(fmt.Sprintf("gotrader.%s", mKey)).Timestamp(mValue.X[i].UnixMilli()).Value(mValue.Y[i]).Build()

			cmds = append(cmds, c)
		}

	}
	exp.redis.DoMulti(context.Background(), cmds...)

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
