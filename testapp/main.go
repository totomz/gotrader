package main

import (
	"github.com/totomz/gotrader"
	"log"
	"time"
)

func init() {
	log.SetFlags(log.Ltime | log.Lshortfile)
}

func main() {

	log.Println("Starting")
	symbl := gotrader.Symbol("FB")
	sday := time.Date(2021, 1, 11, 0, 0, 0, 0, time.Local)

	service := gotrader.Cerbero{

		Signals: &gotrader.Signal{},

		Broker: &gotrader.BacktestBrocker{
			InitialCashUSD: 30000,
		},

		Strategy: &gotrader.SimplePsarStrategy{
			Symbol: symbl,
		},

		DataFeed: &gotrader.IBZippedCSV{
			Symbol: symbl,
			Sday:   sday,
		},

		TimeAggregationFunc: gotrader.NoAggregation,
	}

	println(service.TimeAggregationFunc)
}
