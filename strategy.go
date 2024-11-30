package gotrader

import (
	"log"
	"time"
)

type Strategy interface {
	// Eval evaluate the strategy. candles[0] is the latest, candles[1] is the latest - 1, and so on
	Eval(candles []Candle)
	Initialize(broker *Cerbero)
	// Shutdown is called by Cerbero when there are no more incoming candles.
	Shutdown()
}

// <editor-fold desc="Test Strategy" >

type SimplePsarStrategy struct {
	Symbol Symbol
	// Signals Signal
}

func (s *SimplePsarStrategy) Eval(candles []Candle) {
	// We respect the array: The latest is the newest!
	// candles[0].Time ==> 15:00:00
	// candles[1].Time ==> 15:00:05
	// candles[2].Time ==> 15:00:10
	candle := candles[len(candles)-1]

	log.Printf("%s open:%v close: %v", candle, candle.Open, candle.Close)

	// s.Signals.Append(candle, "psar", psar[len(psar)-1])

}

func (s *SimplePsarStrategy) Initialize(_ *Cerbero) {
	// Strategy initialization
}

// </editor-fold desc="Test Strategy" >

func SameTime(a time.Time, hour, minute, seconds int) bool {
	return a.Hour() == hour &&
		a.Minute() == minute &&
		a.Second() == seconds
}

// StopAt is a usefull function for debug a strategy. Returns true if the time of the candle match the given one
func StopAt(c Candle, hour, min, sec int) bool {
	if SameTime(c.Time, hour, min, sec) {
		return true
	}
	return false
}
