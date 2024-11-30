package gotrader

import (
	"testing"
	"time"
)

// ZigZagTestStrategy test the ZigZag indicator
type ZigZagTestStrategy struct {
	broker      Broker
	EvalHandler func(candles []Candle, s *MockStrategy)
}

func (s *ZigZagTestStrategy) Initialize(cerbero *Cerbero) {
	s.broker = cerbero.Broker
}

func (s *ZigZagTestStrategy) Shutdown() {
}

func (s *ZigZagTestStrategy) Eval(candles []Candle) {
	c := candles[len(candles)-1]

	// ZigZag can be calculated only at theend of the trading day.
	// Which should be a convinient funcion, I guess
	if !SameTime(c.Time, 21, 55, 0) {
		return
	}

	zigzag := ZigZag(candles)

	println(len(zigzag))
	println("ciocchino!")
}

func TestZigZagCalc(t *testing.T) {

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
		Strategy: &ZigZagTestStrategy{},
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
