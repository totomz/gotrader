package gotrader

import (
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

// Eval evaluate the strategy. candles[0] is the latest, candles[1] is the latest - 1, and so on
type MockStrategy struct {
	broker      Broker
	EvalHandler func(candles []Candle, s *MockStrategy)
}

func (s *MockStrategy) Initialize(cerbero *Cerbero) {
	s.broker = cerbero.Broker
}

func (s *MockStrategy) Shutdown() {
}

// A strategy can defines metrics and views
var (
	MTestPsar      = stats.Float64("test/psar", "", stats.UnitDimensionless)
	MTestPSarTrend = stats.Float64("test/psar_trend", "", stats.UnitDimensionless)

	TestMetric = NewMetricWithDefaultViews("psar2")

	testViews = []*view.View{
		{Measure: MTestPsar, Aggregation: view.LastValue(), TagKeys: []tag.Key{KeySymbol}},
		{Measure: MTestPSarTrend, Aggregation: view.LastValue(), TagKeys: []tag.Key{KeySymbol}},
	}
)

func (s *MockStrategy) Eval(candles []Candle) {

	if s.EvalHandler != nil {
		s.EvalHandler(candles, s)
		return
	}

	c := candles[len(candles)-1]
	psar, trend := []float64{0}, []int{-1}
	_ = Open(candles) // this line is useless, I need it to avoid the lint error "Unused function Open"
	ctx := GetNewContextFromCandle(c)
	stats.Record(ctx, MTestPsar.M(psar[len(psar)-1]))
	stats.Record(ctx, MTestPSarTrend.M(float64(trend[len(trend)-1])))

	TestMetric.Record(ctx, psar[len(psar)-1])
	_, _ = TestMetric.Get(ctx, -3)
	if c.Time.Equal(time.Date(2021, 1, 11, 17, 11, 30, 00, time.Local)) {
		_, _ = s.broker.SubmitOrder(c, Order{
			Size:   50,
			Symbol: "FB",
			Type:   OrderBuy,
		})
	}

	if c.Time.Equal(time.Date(2021, 1, 11, 18, 32, 30, 00, time.Local)) {
		_, _ = s.broker.SubmitOrder(c, Order{
			Size:   50,
			Symbol: "FB",
			Type:   OrderSell,
		})
	}

}

func TestSignalsStrategy(t *testing.T) {

	symbl := Symbol("FB")
	sday := time.Date(2021, 1, 11, 0, 0, 0, 0, time.Local)

	// Register the metric views
	RegisterViews(testViews...)

	service := Cerbero{
		// Signals: &signals,
		Broker: &BacktestBrocker{
			BrokerAvailableCash: 30000,
			OrderMap:            map[string]*Order{},
			Portfolio:           map[Symbol]Position{},
			EvalCommissions:     Nocommissions,
		},
		Strategy: &MockStrategy{},
		DataFeed: &IBZippedCSV{
			DataFolder: testFolder,
			Symbol:     symbl,
			Sday:       sday,
		},
		TimeAggregationFunc: AggregateBySeconds(5),
	}

	_, err := service.Run()
	if err != nil {
		t.Errorf("failed to run strategy - %v", err)
	}

}

func TestShortOrders(t *testing.T) {

	symbl := Symbol("FB")
	sday := time.Date(2021, 1, 11, 0, 0, 0, 0, time.Local)
	// signals := &MemorySignals{Metrics: map[string]*TimeSerie{}}

	// Open a short position and close  it
	// Expected to make money
	shortStrategy := MockStrategy{EvalHandler: func(candles []Candle, s *MockStrategy) {
		c := candles[len(candles)-1]
		if c.Time.Equal(time.Date(2021, 1, 11, 17, 11, 30, 00, time.Local)) {
			_, _ = s.broker.SubmitOrder(c, Order{
				Size:   50,
				Symbol: "FB",
				Type:   OrderSell,
			})
		}

		if c.Time.Equal(time.Date(2021, 1, 11, 20, 13, 45, 00, time.Local)) {
			_, _ = s.broker.SubmitOrder(c, Order{
				Size:   50,
				Symbol: "FB",
				Type:   OrderBuy,
			})
		}

	}}

	service := Cerbero{
		Broker: &BacktestBrocker{
			BrokerAvailableCash: 30000,
			OrderMap:            map[string]*Order{},
			Portfolio:           map[Symbol]Position{},
			EvalCommissions:     Nocommissions,
		},
		Strategy: &shortStrategy,
		DataFeed: &IBZippedCSV{
			DataFolder: testFolder,
			Symbol:     symbl,
			Sday:       sday,
		},
		TimeAggregationFunc: AggregateBySeconds(5),
	}

	res, err := service.Run()
	if err != nil {
		t.Errorf("failed to run strategy - %v", err)
	}

	if res.FinalCash-res.InitialCash < 50.0 {
		t.Errorf("expected a gain, got %f", res.FinalCash-res.InitialCash)
	}

}

type mockPlainStrategy struct {
	MockStrategy
}

type mockTradeOnlyStrategy struct {
	MockStrategy
	trades []Trade
}

func (s *mockTradeOnlyStrategy) OnTrade(trade Trade) {
	s.trades = append(s.trades, trade)
}

type mockTradeQuoteStrategy struct {
	MockStrategy
	trades []Trade
	quotes []Quote
}

func (s *mockTradeQuoteStrategy) OnTrade(trade Trade) {
	s.trades = append(s.trades, trade)
}

func (s *mockTradeQuoteStrategy) OnQuote(quote Quote) {
	s.quotes = append(s.quotes, quote)
}

func TestDispatchEvent(t *testing.T) {
	t.Parallel()

	trade := Trade{Ticker: "FB", TS: 1699972200000, Seq: 1, Price: 10.5, Size: 100}
	quote := Quote{Ticker: "FB", TS: 1699972200001, Seq: 2, BidPrice: 10.4, AskPrice: 10.6, BidSize: 10, AskSize: 20}

	events := []MarketEvent{
		{Trade: &trade},
		{Quote: &quote},
		{},
	}

	plain := &mockPlainStrategy{}
	tradeOnly := &mockTradeOnlyStrategy{}
	both := &mockTradeQuoteStrategy{}

	for _, event := range events {
		dispatchEvent(plain, event)
		dispatchEvent(tradeOnly, event)
		dispatchEvent(both, event)
	}

	if diff := cmp.Diff([]Trade{trade}, tradeOnly.trades); diff != "" {
		t.Errorf("trade-only strategy trades mismatch (-want +got):\n%s", diff)
	}

	if diff := cmp.Diff([]Trade{trade}, both.trades); diff != "" {
		t.Errorf("trade+quote strategy trades mismatch (-want +got):\n%s", diff)
	}

	if diff := cmp.Diff([]Quote{quote}, both.quotes); diff != "" {
		t.Errorf("trade+quote strategy quotes mismatch (-want +got):\n%s", diff)
	}
}
