package gotrader

import (
	"context"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
)

var (
	MCandleOpen  = stats.Float64("gotrader/candle/open", "", stats.UnitDimensionless)
	MCandleHigh  = stats.Float64("gotrader/candle/high", "", stats.UnitDimensionless)
	MCandleClose = stats.Float64("gotrader/candle/close", "", stats.UnitDimensionless)
	MCandleLow   = stats.Float64("gotrader/candle/low", "", stats.UnitDimensionless)
	MTradesBuy   = stats.Float64("gotrader/trades/buy", "", stats.UnitDimensionless)
	MTradesSell  = stats.Float64("gotrader/trades/sell", "", stats.UnitDimensionless)
	MCash        = stats.Float64("gotrader/cash", "", stats.UnitDimensionless)
	MPosition    = stats.Float64("gotrader/position", "", stats.UnitDimensionless)

	KeySymbol, _     = tag.NewKey("symbol")
	KeyCandleTime, _ = tag.NewKey("candleTime") // Time in some string representation

	defaulSignalsView = []*view.View{
		{Measure: MCandleOpen, Aggregation: view.LastValue(), TagKeys: []tag.Key{KeySymbol, KeyCandleTime}},
		{Measure: MCandleHigh, Aggregation: view.LastValue(), TagKeys: []tag.Key{KeySymbol, KeyCandleTime}},
		{Measure: MCandleClose, Aggregation: view.LastValue(), TagKeys: []tag.Key{KeySymbol, KeyCandleTime}},
		{Measure: MCandleLow, Aggregation: view.LastValue(), TagKeys: []tag.Key{KeySymbol, KeyCandleTime}},
		{Measure: MCandleLow, Aggregation: view.LastValue(), TagKeys: []tag.Key{KeySymbol, KeyCandleTime}},
	}
)

func GetNewContextFromCandle(c Candle) context.Context {
	ctx, err := tag.New(context.Background(),
		tag.Insert(KeySymbol, string(c.Symbol)),
		tag.Insert(KeyCandleTime, c.Time.String()),
	)
	if err != nil {
		panic(err) // This should never happen, really
	}

	return ctx
}

// // Signal is a convenient way to collect custom time-series
// type Signal interface {
// 	// Append the value as last element of this serie
// 	Append(candle Candle, name string, value float64)
//
// 	// Get the i-th element back  (Get(0) return the last element, Get(1) return the last - 1 element)
// 	Get(candle Candle, name string, i int) (float64, error)
//
// 	// Flush the metrics to the underlying system
// 	// This method is called before a new candle is processed by cerbero
// 	Flush()
//
// 	GetMetrics() map[string]*TimeSerie
// }
//
// type NoOpSignals struct {
// }
//
// type MemorySignals struct {
// 	Metrics map[string]*TimeSerie
// }
//
// // Append a metric to a given signal.
// func (s *MemorySignals) Append(candle Candle, name string, value float64) {
//
// 	if s.Metrics == nil {
// 		s.Metrics = map[string]*TimeSerie{}
// 	}
//
// 	key := string(candle.Symbol) + "." + name
// 	_, exists := s.Metrics[key]
// 	if !exists {
// 		s.Metrics[key] = &TimeSerie{
// 			X: []time.Time{},
// 			Y: []float64{},
// 		}
// 	}
//
// 	s.Metrics[key].Append(candle, value)
//
// }
//
// func (s *MemorySignals) Flush() {
//
// }
//
// func (s *MemorySignals) Get(candle Candle, name string, i int) (float64, error) {
// 	ts, found := s.Metrics[string(candle.Symbol)+"."+name]
// 	if !found {
// 		return 0, errors.New("signal not found")
// 	}
//
// 	index := len(ts.Y) - 1 - i
// 	if index < 0 {
// 		return 0, errors.New("index too old")
// 	}
//
// 	return ts.Y[index], nil
// }
//
// func (s *MemorySignals) GetMetrics() map[string]*TimeSerie {
// 	return s.Metrics
// }
//
// type TimeSerie struct {
// 	X []time.Time `json:"x"`
// 	Y []float64   `json:"y"`
// }
//
// // Append an element to the end of this ts
// func (ts *TimeSerie) Append(candle Candle, value float64) {
// 	ts.X = append(ts.X, candle.Time)
// 	ts.Y = append(ts.Y, value)
// }
//
// type PlotlyTimeSerieCandle struct {
// 	X      []int64   `json:"x"`
// 	Open   []float64 `json:"open"`
// 	High   []float64 `json:"high"`
// 	Close  []float64 `json:"close"`
// 	Low    []float64 `json:"low"`
// 	Volume []int64   `json:"volume"`
// }
//
// type PlotlyTimeSerieFloat struct {
// 	X []int64   `json:"x"`
// 	Y []float64 `json:"y"`
// }
//
// func SignalsToPlotly(symbol string, signal Signal) []byte {
//
// 	data := struct {
// 		values map[string]interface{}
// 	}{
// 		values: map[string]interface{}{},
// 	}
//
// 	candles := PlotlyTimeSerieCandle{
// 		X:      []int64{},
// 		Open:   []float64{},
// 		High:   []float64{},
// 		Close:  []float64{},
// 		Low:    []float64{},
// 		Volume: []int64{},
// 	}
//
// 	for k, values := range signal.GetMetrics() {
//
// 		if k == fmt.Sprintf("%s.candle_open", symbol) {
// 			for i, inst := range values.X {
// 				candles.X = append(candles.X, inst.UnixMilli())
// 				candles.Open = append(candles.Open, values.Y[i])
// 			}
// 		} else if k == fmt.Sprintf("%s.candle_high", symbol) {
// 			for _, v := range values.Y {
// 				candles.High = append(candles.High, v)
// 			}
// 		} else if k == fmt.Sprintf("%s.candle_close", symbol) {
// 			for _, v := range values.Y {
// 				candles.Close = append(candles.Close, v)
// 			}
// 		} else if k == fmt.Sprintf("%s.candle_low", symbol) {
// 			for _, v := range values.Y {
// 				candles.Low = append(candles.Low, v)
// 			}
// 		} else if k == fmt.Sprintf("%s.candle_volume", symbol) {
// 			for _, v := range values.Y {
// 				candles.Volume = append(candles.Volume, int64(v))
// 			}
// 		} else {
// 			name := strings.Split(k, ".")[1]
// 			ts := PlotlyTimeSerieFloat{
// 				X: []int64{},
// 				Y: []float64{},
// 			}
//
// 			for i, inst := range values.X {
// 				ts.X = append(ts.X, inst.UnixMilli())
// 				ts.Y = append(ts.Y, values.Y[i])
// 			}
//
// 			data.values[name] = ts
// 		}
//
// 		data.values["candles"] = candles
//
// 	}
//
// 	_, existsBuy := data.values["trades_buy"]
// 	if !existsBuy {
// 		data.values["trades_buy"] = PlotlyTimeSerieFloat{
// 			X: []int64{},
// 			Y: []float64{},
// 		}
// 	}
//
// 	_, existsSell := data.values["trades_sell"]
// 	if !existsSell {
// 		data.values["trades_sell"] = PlotlyTimeSerieFloat{
// 			X: []int64{},
// 			Y: []float64{},
// 		}
// 	}
//
// 	res, err := json.Marshal(data.values)
// 	if err != nil {
// 		log.Printf("[ERROR] error exporting signals: %v ", err)
// 		return []byte{}
// 	}
// 	/*
// 		data := {
// 			candles: {
// 				x: [],
// 				open: [],
// 				close: []
// 				....
// 			}
// 			<signal>: {
// 				x: ..
// 			}
// 		}
// 	*/
//
// 	return res
// }
//
// func SignalsToGrafana(signal Signal) []byte {
//
// 	m := signal.GetMetrics()
// 	b, err := json.Marshal(m)
// 	if err != nil {
// 		panic(err)
// 	}
// 	return b
// }
