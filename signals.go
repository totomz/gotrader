package gotrader

import (
	"time"
)

// Signal is a convenient way to collect custom time-series
type Signal struct {
	Metrics map[string]*TimeSerie
}

// Append a metric to a given signal.
func (s *Signal) Append(candle Candle, name string, value float64) {

	if s.Metrics == nil {
		s.Metrics = map[string]*TimeSerie{}
	}

	key := string(candle.Symbol) + "." + name
	_, exists := s.Metrics[key]
	if !exists {
		s.Metrics[key] = &TimeSerie{
			X: []time.Time{},
			Y: []float64{},
		}
	}

	s.Metrics[key].Append(candle, value)

}

type TimeSerie struct {
	X []time.Time
	Y []float64
}

// Append an element to the end of this ts
func (ts *TimeSerie) Append(candle Candle, value float64) {
	ts.X = append(ts.X, candle.Time)
	ts.Y = append(ts.Y, value)
}

func SignalsToPlotly(signal Signal) []byte {

	panic("TODO")
	// data, err := json.Marshal(signal.Metrics)
	// if err != nil {
	// 	log.Printf("[ERROR] error exporting signals: %v ", err)
	// 	return []byte{}
	// }
	/*
		data := {
			candles: {
				x: [],
				open: [],
				close: []
				....
			}
			<signal>: {
				x: ..
			}
		}
	*/
}
