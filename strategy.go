package gotrader

import (
	"github.com/cinar/indicator"
	"log"
)

type Strategy interface {
	// Eval evaluate the strategy. candles[0] is the latest, candles[1] is the latest - 1, and so on
	Eval(candles []Candle)
	Initialize(broker *Cerbero)
}

// <editor-fold desc="Test Strategy" >

type SimplePsarStrategy struct {
	Symbol  Symbol
	Signals Signal
}

func (s *SimplePsarStrategy) Eval(candles []Candle) {
	// We respect the array: The latest is the newest!
	// candles[0].Time ==> 15:00:00
	// candles[1].Time ==> 15:00:05
	// candles[2].Time ==> 15:00:10
	candle := candles[len(candles)-1]

	psar, trend := indicator.ParabolicSar(High(candles), Low(candles), Close(candles))
	log.Printf("%s psar:%v trend: %v", candle, psar[len(psar)-1], trend[len(trend)-1])

	s.Signals.Append(candle, "psar", psar[len(psar)-1])

}

func (s *SimplePsarStrategy) Initialize(_ *Cerbero) {
	// Strategy initialization
}

// </editor-fold desc="Test Strategy" >
