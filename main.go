package main

import (
	"github.com/totomz/autotrader/gotrader"
	"log"
	"time"
)

func init() {
	log.SetFlags(log.Ltime | log.Lshortfile)
}

func main() {

	log.Println("Starting")

	service := gotrader.Cerbero{

		Broker: &gotrader.BacktestBrocker{
			InitialCashUSD: 30000,
		},

		Strategy: &gotrader.SimplePsarStrategy{
			Symbol: gotrader.Symbol("AMZN"),
		},

		DataFeed: &gotrader.IBZippedCSV{
			Symbol: gotrader.Symbol("AMZN"),
			Sday:   time.Date(2021, 1, 11, 0, 0, 0, 0, time.Local),
		},

		TimeAggregationFunc: gotrader.NoAggregation,
	}

	println(service.TimeAggregationFunc)
}
