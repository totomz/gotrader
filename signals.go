package gotrader

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
)

// Signal is a convenient way to collect custom time-series
type Signal interface {
	Append(candle Candle, name string, value float64)

	// Flush the metrics to the underlying system
	// This method is called before a new candle is processed by cerbero
	Flush()

	GetMetrics() map[string]*TimeSerie
}

type MemorySignals struct {
	Metrics map[string]*TimeSerie
}

// Append a metric to a given signal.
func (s *MemorySignals) Append(candle Candle, name string, value float64) {

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

func (s *MemorySignals) Flush() {

}

func (s *MemorySignals) GetMetrics() map[string]*TimeSerie {
	return s.Metrics
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

type PlotlyTimeSerieCandle struct {
	X      []int64   `json:"x"`
	Open   []float64 `json:"open"`
	High   []float64 `json:"high"`
	Close  []float64 `json:"close"`
	Low    []float64 `json:"low"`
	Volume []int64   `json:"volume"`
}

type PlotlyTimeSerieFloat struct {
	X []int64   `json:"x"`
	Y []float64 `json:"y"`
}

func SignalsToPlotly(symbol string, signal Signal) []byte {

	data := struct {
		values map[string]interface{}
	}{
		values: map[string]interface{}{},
	}

	candles := PlotlyTimeSerieCandle{
		X:      []int64{},
		Open:   []float64{},
		High:   []float64{},
		Close:  []float64{},
		Low:    []float64{},
		Volume: []int64{},
	}

	for k, values := range signal.GetMetrics() {

		if k == fmt.Sprintf("%s.candle_open", symbol) {
			for i, inst := range values.X {
				candles.X = append(candles.X, inst.UnixMilli())
				candles.Open = append(candles.Open, values.Y[i])
			}
		} else if k == fmt.Sprintf("%s.candle_high", symbol) {
			for _, v := range values.Y {
				candles.High = append(candles.High, v)
			}
		} else if k == fmt.Sprintf("%s.candle_close", symbol) {
			for _, v := range values.Y {
				candles.Close = append(candles.Close, v)
			}
		} else if k == fmt.Sprintf("%s.candle_low", symbol) {
			for _, v := range values.Y {
				candles.Low = append(candles.Low, v)
			}
		} else if k == fmt.Sprintf("%s.candle_volume", symbol) {
			for _, v := range values.Y {
				candles.Volume = append(candles.Volume, int64(v))
			}
		} else {
			name := strings.Split(k, ".")[1]
			ts := PlotlyTimeSerieFloat{
				X: []int64{},
				Y: []float64{},
			}

			for i, inst := range values.X {
				ts.X = append(ts.X, inst.UnixMilli())
				ts.Y = append(ts.Y, values.Y[i])
			}

			data.values[name] = ts
		}

		data.values["candles"] = candles

	}

	_, existsBuy := data.values["trades_buy"]
	if !existsBuy {
		data.values["trades_buy"] = PlotlyTimeSerieFloat{
			X: []int64{},
			Y: []float64{},
		}
	}

	_, existsSell := data.values["trades_sell"]
	if !existsSell {
		data.values["trades_sell"] = PlotlyTimeSerieFloat{
			X: []int64{},
			Y: []float64{},
		}
	}

	res, err := json.Marshal(data.values)
	if err != nil {
		log.Printf("[ERROR] error exporting signals: %v ", err)
		return []byte{}
	}
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

	return res
}
