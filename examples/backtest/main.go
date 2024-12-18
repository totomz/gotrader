package main

import (
	"fmt"
	"github.com/totomz/gotrader"
	"go.opencensus.io/stats/view"
	"time"
)

// SimpleStrategy is the struct containing your strategy that implements the gotrader.Strategy interface
type SimpleStrategy struct {
	// broker hold a reference to the current broker, to get current positions and execute orders
	broker gotrader.Broker
}

func (s *SimpleStrategy) Shutdown() {
}

func (s *SimpleStrategy) Initialize(cerbero *gotrader.Cerbero) {
	s.broker = cerbero.Broker
}

// Eval is called each time a new candle is ready. The
func (s *SimpleStrategy) Eval(candles []gotrader.Candle) {

	// c is the latest candle
	c := candles[len(candles)-1]

	// Calculate indicators
	psar := 0.0
	currentPosition := s.broker.GetPosition(c.Symbol)

	// buy if we're not in a position
	if currentPosition.Size == 0 {
		if psar > c.Close {
			_, err := s.broker.SubmitOrder(c, gotrader.Order{
				Size:   10,
				Symbol: c.Symbol,
				Type:   gotrader.OrderBuy,
			})

			if err != nil {
				// handle the error
			}

		}

		return
	} else {
		_, err := s.broker.SubmitOrder(c, gotrader.Order{
			Size:   10,
			Symbol: c.Symbol,
			Type:   gotrader.OrderSell,
		})

		if err != nil {
			// handle the error
		}
	}

}

// main Example of a backtesting strategy
func main() {

	symbl := gotrader.Symbol("FB")
	sday := time.Date(2021, 1, 11, 0, 0, 0, 0, time.Local)

	exporter, err := gotrader.NewRedisExporter("127.0.0.1:6379")
	view.RegisterExporter(exporter)

	// Disable the automatic metric reporting whn running in backtest
	// and call Flush() (see at the end)
	view.SetReportingPeriod(1 * time.Hour)

	/*
		It is possible to override the global logger by setting these variables
		eg: in backtesting, is useful to log to io.Discard to skip logs and run faster
		gotrader.Stdout = log.New(io.Discard, "", log.Lshortfile|log.Ltime|log.Lmsgprefix)
		gotrader.Stderr = log.New(os.Stdout, "[ERROR]", log.Lshortfile|log.Ltime|log.Lmsgprefix)
	*/

	startingCash := 10000.0
	service := gotrader.Cerbero{
		Broker: &gotrader.BacktestBrocker{
			OrderMap:            map[string]*gotrader.Order{},
			Portfolio:           map[gotrader.Symbol]gotrader.Position{},
			BrokerAvailableCash: startingCash,
			EvalCommissions:     gotrader.Nocommissions,
		},
		Strategy: &SimpleStrategy{}, // your strategy to run
		DataFeed: &gotrader.IBZippedCSV{ // candle datafeed; CSV files for backtesting
			Symbol:     symbl,
			Sday:       sday,
			DataFolder: "./datasets",
			Slowtime:   0 * time.Second, // Wait before sending out candles; set to 0 to run at full speed in backtesting
		},
		TimeAggregationFunc: gotrader.AggregateBySeconds(10),
	}

	result, err := service.Run()
	if err != nil {
		panic(err)
	}

	println(fmt.Sprintf("Run in %s; strategy result: %f", result.TotalTimeString, startingCash-result.FinalCash))
	start := time.Now()
	exporter.Flush() // When running in backtest, you need to flush all the metrics (reporter could not

	println(fmt.Sprintf("elapsed flush: %v", time.Now().Sub(start)))
}
