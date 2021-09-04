package gotrader

import (
	"github.com/google/go-cmp/cmp"
	"os"
	"sort"
	"testing"
	"time"
)

// Eval evaluate the strategy. candles[0] is the latest, candles[1] is the latest - 1, and so on
type MockStrategy struct {
	signals Signal
}

func (s *MockStrategy) Initialize(cerbero *Cerbero) {
	s.signals = cerbero.signals
}
func (s *MockStrategy) Eval(candles []Candle) {
	c := candles[len(candles)-1]
	s.signals.AppendFloat(c, "pippo", c.High)
}

func TestSignalsStrategy(t *testing.T) {

	symbl := Symbol("FB")
	sday := time.Date(2021, 1, 11, 0, 0, 0, 0, time.Local)

	service := Cerbero{
		Broker:   NewBacktestBrocker(30000),
		Strategy: &MockStrategy{},
		DataFeed: &IBZippedCSV{
			DataFolder: testFolder,
			Symbol:     symbl,
			Sday:       sday,
		},
		TimeAggregationFunc: AggregateBySeconds(5),
	}

	err := service.Run()
	if err != nil {
		t.Errorf("failed to run strategy - %v", err)
	}

	// The strategy exposes custom signals through cerbero
	signals := service.Signals().values

	// Candles, trades, cash are always added to the signals
	wantedKeys := []string{SIGNAL_CASH, SIGNAL_TRADES, SIGNAL_CANDLES, "pippo"}
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
	cash, found := signals[SIGNAL_CASH].(TimeSerieFloat)
	if !found {
		t.Error("signal CASH not found!")
	}
	for _, val := range cash.Y {
		if val != 30000 {
			t.Error("cash has changed?")
		}
	}

	_, found = signals["pippo"]
	if !found {
		t.Error("signal pippo not found!")
	}

	// get the JSON
	daje, err := service.Signals().ToJson()

	// Write the file
	err = os.WriteFile("./plotly/datatest.json", daje, 0644)
	if err != nil {
		t.Error(err)
	}

}
