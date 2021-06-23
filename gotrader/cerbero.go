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

type Cerbero struct {
	Broker              Broker
	Strategy            Strategy
	DataFeed            DataFeed
	TimeAggregationFunc TimeAggregation
}
