package gotrader

import (
	"encoding/json"
	"log"
)

// Signal is a convenient way to collect custom time-series
type Signal struct {
	values map[string]interface{}
}

type TimeSerieCandle struct {
	X      []int64   `json:"x"`
	Open   []float64 `json:"open"`
	High   []float64 `json:"high"`
	Close  []float64 `json:"close"`
	Low    []float64 `json:"low"`
	Volume []int64   `json:"volume"`
}

type TimeSerieFloat struct {
	X []int64   `json:"x"`
	Y []float64 `json:"y"`
}

// AppendCandle append a new value to a time-series. The reference time is taken from Candle time.
// It is not possible to append values in the past; this function is append-only
func (signal *Signal) AppendCandle(candle Candle, key string, value Candle) {
	// Init values before appending
	if signal.values == nil {
		signal.values = map[string]interface{}{}
	}

	_, found := signal.values[key]
	if !found {
		signal.values[key] = TimeSerieCandle{
			// X:      []string{candle.Time.Format("2006-01-02 15:04:05")},
			X:      []int64{candle.Time.Unix() * 1000},
			Open:   []float64{value.Open},
			High:   []float64{value.High},
			Close:  []float64{value.Close},
			Low:    []float64{value.Low},
			Volume: []int64{value.Volume},
		}
		return
	}

	cval := signal.values[key].(TimeSerieCandle)
	// cval.X = append(cval.X, candle.Time.Format("2006-01-02 15:04:05"))
	cval.X = append(cval.X, candle.Time.Unix()*1000)
	cval.Open = append(cval.Open, value.Open)
	cval.High = append(cval.High, value.High)
	cval.Close = append(cval.Close, value.Close)
	cval.Low = append(cval.Open, value.Low)
	cval.Volume = append(cval.Volume, value.Volume)
	signal.values[key] = cval
}

func (signal *Signal) AppendNil(candle Candle, key string, value float64) {

}

func (signal *Signal) AppendFloat(candle Candle, key string, value float64) {

	// Init values before appending
	if signal.values == nil {
		signal.values = map[string]interface{}{}
	}

	_, found := signal.values[key]
	if !found {
		signal.values[key] = TimeSerieFloat{
			X: []int64{candle.Time.Unix() * 1000},
			Y: []float64{value},
		}
		return
	}

	cval := signal.values[key].(TimeSerieFloat)
	cval.X = append(cval.X, candle.Time.Unix()*1000)
	cval.Y = append(cval.Y, value)
	signal.values[key] = cval
}

func (signal *Signal) Keys() []string {
	var keys []string
	for k, _ := range signal.values {
		keys = append(keys, k)
	}

	return keys
}

func (signal *Signal) Get(key string) (interface{}, bool) {
	vals, found := signal.values[key]
	return vals, found
}

func (signal *Signal) ToJson() ([]byte, error) {

	// Append the default signals
	requiredSignals := []string{"cash", "trades_buy", "trades_sell", "candles"}
	for _, k := range requiredSignals {
		_, has := signal.values[k]
		if !has {
			signal.values[k] = TimeSerieFloat{
				X: []int64{},
				Y: []float64{},
			}
		}
	}

	data, err := json.Marshal(signal.values)
	if err != nil {
		log.Printf("[ERROR] error exporting signals: %v ", err)
		return []byte{}, err
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

	return data, nil
}

func (signal *Signal) Merge(toMerge *Signal) {
	if toMerge == nil {
		return
	}

	for k, v := range toMerge.values {
		signal.values[k] = v
	}
}
