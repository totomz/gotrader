package gotrader

import (
	"log"
	"time"
)

type Candle struct {
	Open   float64
	High   float64
	Close  float64
	Low    float64
	Volume float64
	Time   time.Time
}

// DataFeed provides a stream of Candle.
type DataFeed interface {

	// Run starts a go routine that poll the data source, and push the candles in the returned channel.
	// The channel is expected to have a buffer larger enough to handle 1 day of data
	Run() (chan Candle, error)
}

// <editor-fold desc="IBZippedCSV" >

type IBZippedCSV struct {
	Sday   time.Time
	Symbol Symbol
}

func (d *IBZippedCSV) Run() (chan Candle, error) {
	log.Fatal("NOT IMPLEMENTED")
	return nil, nil
}

// </editor-fold>
