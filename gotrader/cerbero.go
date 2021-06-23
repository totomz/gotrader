package gotrader

// TimeAggregation aggregate the candles from a channel and write the output in a separate channel
type TimeAggregation func(<-chan Candle) <-chan Candle

func NoAggregation(inputCandleChan <-chan Candle) <-chan Candle {
	outchan := make(chan Candle, 1)

	go func() {
		defer close(outchan)
		i := 0
		for candle := range inputCandleChan {
			i = i + 1
			outchan <- candle
		}
	}()
	return outchan
}

func AggregateBySeconds(sec int) TimeAggregation {
	return func(inputCandleChan <-chan Candle) <-chan Candle {
		outchan := make(chan Candle, 10000)

		go func() {
			defer close(outchan)
			i := 0
			aggregated := Candle{}
			for candle := range inputCandleChan {
				aggregated = mergeCandles(aggregated, candle)
				if i == sec {
					outchan <- aggregated
					aggregated = Candle{}
					i = 0
				}
				i = i + 1
			}
		}()
		return outchan
	}
}

// mergeCandles suppose that a is before b.
func mergeCandles(a Candle, b Candle) Candle {
	merged := Candle{}

	merged.Open = a.Open
	if a.Open == 0 {
		merged.Open = b.Open
	}

	merged.Close = b.Close

	if a.High > b.High {
		merged.High = a.High
	} else {
		merged.High = b.High
	}

	if a.Low > 0 && a.Low < b.Low {
		merged.Low = a.Low
	} else {
		merged.Low = b.Low
	}

	merged.Time = b.Time

	merged.Volume = a.Volume + b.Volume

	return merged
}

type Cerbero struct {
	Broker              Broker
	Strategy            Strategy
	DataFeed            DataFeed
	TimeAggregationFunc TimeAggregation
}
