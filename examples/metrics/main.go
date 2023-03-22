package main

import (
	"github.com/cinar/indicator"
	"github.com/totomz/gotrader/gotrader"
	"log"
	"os"
	"time"
)

var (
	// Psar is a custom metric.
	// metrics are implemented as Opencesus metrics, and can be exported to a local grafana instance using https://github.com/go-trader/gotaset
	Psar = gotrader.NewMetricWithDefaultViews("psar")
)

type EmptyStrategy struct{}

func (s *EmptyStrategy) Initialize(_ *gotrader.Cerbero) {}

// Eval is called each time a new candle is ready. The
func (s *EmptyStrategy) Eval(candles []gotrader.Candle) {
	c := candles[len(candles)-1]
	psar, _ := indicator.ParabolicSar(gotrader.High(candles), gotrader.Low(candles), gotrader.Close(candles))

	// Metrics/signals are associated to a context,
	// this way we can link a metric to the symbol it belongs to
	ctx := gotrader.GetNewContextFromCandle(c)
	Psar.Record(ctx, psar[len(psar)-1])
}

// main Example of a backtesting strategy
func main() {

	// The creation of a service and datafeed is out of context for this example
	_, err := boringStuff()
	if err != nil {
		panic(err)
	}

	// When running in realtime,
	// metrics are exported by regular Opencensus exporter
	// When backtesting, the timestamps are in the past (time is given by the candle)
	// We can export the metrics to a json file, and feed it in a grafana
	dataGrafana := gotrader.SignalsToGrafana()
	err = os.WriteFile("plotly/signals_grafana.json", dataGrafana, 0644)
	if err != nil {
		panic(err)
	}

	// Now, run `docker-compose up` and go to http://localhost:3000
	// Data have the time of the candles! This example uses candles from 11/01/2021 ( <-- 11 January),
	// if you don't see any data in the chart, change the time range

}

func boringStuff() (gotrader.ExecutionResult, error) {
	symbl := gotrader.Symbol("FB")
	sday := time.Date(2021, 1, 11, 0, 0, 0, 0, time.Local)
	stdout := log.New(os.Stdout, "", log.Lshortfile|log.Ltime|log.Lmsgprefix)
	stderr := log.New(os.Stdout, "[ERROR]", log.Lshortfile|log.Ltime|log.Lmsgprefix)

	startingCash := 10000.0
	service := gotrader.Cerbero{
		Broker: &gotrader.BacktestBrocker{
			OrderMap:            map[string]*gotrader.Order{},
			Portfolio:           map[gotrader.Symbol]gotrader.Position{},
			BrokerAvailableCash: startingCash,
			EvalCommissions:     gotrader.Nocommissions,
			Stdout:              stdout,
			Stderr:              stderr,
		},
		Strategy: &EmptyStrategy{}, // your strategy to run
		DataFeed: &gotrader.IBZippedCSV{ // candle datafeed; CSV files for backtesting
			Symbol:     symbl,
			Sday:       sday,
			DataFolder: "./datasets",
		},
		TimeAggregationFunc: gotrader.AggregateBySeconds(10),

		Stdout: stdout, // this could be set to nil to avoid logging in backtesting
		Stderr: stderr, // errors can have their custom logger
	}

	return service.Run()
}
