package gotrader

import (
	"encoding/json"
)

// Signal is a convenient way to collect custom time-series
type Signal struct {
	values map[string]interface{}
}

type TimeSerieCandle struct {
	X      []string  `json:"x"`
	Open   []float64 `json:"open"`
	High   []float64 `json:"high"`
	Close  []float64 `json:"close"`
	Low    []float64 `json:"low"`
	Volume []int64   `json:"volume"`
}

type TimeSerieFloat struct {
	X []string  `json:"x"`
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
			X:      []string{candle.Time.Format("2006-01-02 15:04:05")},
			Open:   []float64{value.Open},
			High:   []float64{value.High},
			Close:  []float64{value.Close},
			Low:    []float64{value.Low},
			Volume: []int64{value.Volume},
		}
		return
	}

	cval := signal.values[key].(TimeSerieCandle)
	cval.X = append(cval.X, candle.Time.Format("2006-01-02 15:04:05"))
	cval.Open = append(cval.Open, value.Open)
	cval.High = append(cval.High, value.High)
	cval.Close = append(cval.Close, value.Close)
	cval.Low = append(cval.Open, value.Low)
	cval.Volume = append(cval.Volume, value.Volume)
	signal.values[key] = cval
}

func (signal *Signal) AppendFloat(candle Candle, key string, value float64) {
	// Init values before appending
	if signal.values == nil {
		signal.values = map[string]interface{}{}
	}

	_, found := signal.values[key]
	if !found {
		signal.values[key] = TimeSerieFloat{
			X: []string{candle.Time.Format("2006-01-02 15:04:05")},
			Y: []float64{value},
		}
		return
	}

	cval := signal.values[key].(TimeSerieFloat)
	cval.X = append(cval.X, candle.Time.Format("2006-01-02 15:04:05"))
	cval.Y = append(cval.Y, value)
	signal.values[key] = cval
}

//// AppendLast the last element of value as the last element to the 'key' time-series. See AppendValue
//func (signal *Signal) AppendLast(candle Candle, key string, value []float64) {
//
//	if len(value) == 0 {
//		return
//	}
//	signal.AppendValue(candle, key, value[len(value) - 1])
//
//}

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

	//type plotlyCandle struct {
	//	X []string `json:"x"`
	//	Open []float64 `json:"open"`
	//	High []float64 `json:"high"`
	//	Close []float64 `json:"close"`
	//	Low []float64 `json:"low"`
	//	Volume []int64 `json:"volume"`
	//}
	//plotlyCandles := plotlyCandle{}
	//
	//candles := signal.values[SIGNAL_CANDLES]
	//for _, c := range candles {
	//	// Mon Jan 2 15:04:05 -0700 MST 2006
	//	plotlyCandles.X = append(plotlyCandles.X, c.Inst.Format("2006-01-02 15:04:05"))	// yyyy-mm-dd HH:MM:SS
	//	plotlyCandles.Open = append(plotlyCandles.Open, c.Value.(Candle).Open)
	//	plotlyCandles.High = append(plotlyCandles.High, c.Value.(Candle).High)
	//	plotlyCandles.Low = append(plotlyCandles.Low, c.Value.(Candle).Low)
	//	plotlyCandles.Close = append(plotlyCandles.Close, c.Value.(Candle).Close)
	//	plotlyCandles.Volume = append(plotlyCandles.Volume, c.Value.(Candle).Volume)
	//
	//}
	//
	///// VOLUMES
	//type plotlyVolume struct {
	//	X []string `json:"x"`
	//	Y []int64 `json:"y"`
	//}
	//plotlyVolumeSerie := plotlyVolume{}
	//for _, c := range candles {
	//	// Mon Jan 2 15:04:05 -0700 MST 2006
	//	plotlyVolumeSerie.X = append(plotlyVolumeSerie.X, c.Inst.Format("2006-01-02 15:04:05"))	// yyyy-mm-dd HH:MM:SS
	//	plotlyVolumeSerie.Y = append(plotlyVolumeSerie.Y, c.Value.(Candle).Volume)
	//}
	//
	//
	////type plotlyResults struct {
	////	Candles plotlyCandle `json:"candles"`
	////	Volume plotlyVolume `json:"volume"`
	////}
	////result := plotlyResults{
	////	Candles: plotlyCandles,
	////	Volume: plotlyVolumeSerie,
	////}
	//result := map[string]interface{}{}
	//for key, values := range signal.values {
	//	result[key] = values
	//}
	//result["candles"] = plotlyCandles
	//result["volume"] = plotlyVolumeSerie
	////result := map[string]interface{}{
	////	"candles": plotlyCandles,
	////	"volume": plotlyVolumeSerie,
	////}

	data, err := json.Marshal(signal.values)
	if err != nil {
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

	println(string(data))
	return data, nil
}
