package gotrader

import (
	"github.com/cinar/indicator"
	"github.com/google/go-cmp/cmp"
	"os"
	"sort"
	"sync"
	"testing"
	"time"
)

// Eval evaluate the strategy. candles[0] is the latest, candles[1] is the latest - 1, and so on
type MockStrategy struct {
	signals     Signal
	broker      Broker
	EvalHandler func(candles []Candle, s *MockStrategy)
}

func (s *MockStrategy) Initialize(cerbero *Cerbero) {
	s.signals = cerbero.signals
	s.broker = cerbero.Broker
}

func (s *MockStrategy) Signals() *Signal {
	return nil
}

func (s *MockStrategy) Eval(candles []Candle) {

	if s.EvalHandler != nil {
		s.EvalHandler(candles, s)
		return
	}

	c := candles[len(candles)-1]
	psar, trend := indicator.ParabolicSar(High(candles), Low(candles), Close(candles))
	s.signals.AppendFloat(c, "psar", psar[len(psar)-1])
	s.signals.AppendFloat(c, "psar_trend", float64(trend[len(trend)-1]))

	if c.Time.Equal(time.Date(2021, 1, 11, 17, 11, 30, 00, time.Local)) {
		_, _ = s.broker.SubmitOrder(Order{
			Size:   50,
			Symbol: "FB",
			Type:   OrderBuy,
		})
	}

	if c.Time.Equal(time.Date(2021, 1, 11, 18, 32, 30, 00, time.Local)) {
		_, _ = s.broker.SubmitOrder(Order{
			Size:   50,
			Symbol: "FB",
			Type:   OrderSell,
		})
	}

}

func TestSignalsStrategy(t *testing.T) {

	symbl := Symbol("FB")
	sday := time.Date(2021, 1, 11, 0, 0, 0, 0, time.Local)

	service := Cerbero{
		Broker: &BacktestBrocker{
			InitialCashUSD:      30000,
			BrokerAvailableCash: 30000,
			OrderMap:            sync.Map{},
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

	// The strategy exposes custom signals through cerbero
	signals := service.Signals().values

	// Candles, trades, cash are always added to the signals
	wantedKeys := []string{SIGNAL_CASH, SIGNAL_TRADES_SELL, SIGNAL_TRADES_BUY, SIGNAL_CANDLES, "psar", "psar_trend"}
	var gotKeys []string
	for k, _ := range signals {
		gotKeys = append(gotKeys, k)
	}

	sort.Strings(wantedKeys)
	sort.Strings(gotKeys)

	if diff := cmp.Diff(wantedKeys, gotKeys); diff != "" {
		t.Errorf("Key() mismatch (-want +got):\n%s", diff)
	}

	// Standard signals
	_, found := signals[SIGNAL_CASH].(TimeSerieFloat)
	if !found {
		t.Error("signal CASH not found!")
	}

	// get the JSON
	daje, err := service.Signals().ToJson()

	// Write the file
	err = os.WriteFile("./plotly/datatest.json", daje, 0644)
	if err != nil {
		t.Error(err)
	}

}

func TestShortOrders(t *testing.T) {

	symbl := Symbol("FB")
	sday := time.Date(2021, 1, 11, 0, 0, 0, 0, time.Local)

	// Open a short position and close  it
	// Expected to make money
	shortStrategy := MockStrategy{EvalHandler: func(candles []Candle, s *MockStrategy) {
		c := candles[len(candles)-1]
		if c.Time.Equal(time.Date(2021, 1, 11, 17, 11, 30, 00, time.Local)) {
			_, _ = s.broker.SubmitOrder(Order{
				Size:   50,
				Symbol: "FB",
				Type:   OrderSell,
			})
		}

		if c.Time.Equal(time.Date(2021, 1, 11, 20, 13, 45, 00, time.Local)) {
			_, _ = s.broker.SubmitOrder(Order{
				Size:   50,
				Symbol: "FB",
				Type:   OrderBuy,
			})
		}
	}}

	service := Cerbero{
		Broker: &BacktestBrocker{
			InitialCashUSD:      30000,
			BrokerAvailableCash: 30000,
			OrderMap:            sync.Map{},
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

	if res.FinalCash-res.InitialCash != 53.5 {
		t.Errorf("expected a gain, got %f", res.FinalCash-res.InitialCash)
	}

}
