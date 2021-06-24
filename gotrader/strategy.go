package gotrader

import "log"

type Strategy interface {
	// Eval evaluate the strategy. candles[0] is the latest, candles[1] is the latest - 1, and so on
	Eval(candles []Candle)
}

// <editor-fold desc="Test Strategy" >

type SimplePsarStrategy struct {
	Symbol Symbol
}

func (s *SimplePsarStrategy) Eval(candles []Candle) {
	log.Fatal("NOT IMPLEMENTED")
}

// </editor-fold desc="Test Strategy" >
