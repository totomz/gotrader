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

		Broker:              &gotrader.BacktestBrocker{},
		TimeAggregationFunc: gotrader.NoAggregation,
		Strategy: &gotrader.SimplePsarStrategy{
			Symbol: symbl,
		},
		DataFeed: &gotrader.IBZippedCSV{
			Symbol: symbl,
			Sday:   sday,
		},
	}

	println(service.TimeAggregationFunc)
}
