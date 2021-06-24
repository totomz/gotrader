package gotrader

import (
	"log"
	"sync"
)

// TimeAggregation aggregate the candles from a channel and write the output in a separate channel
type TimeAggregation func(<-chan Candle) <-chan Candle

func NoAggregation(inputCandleChan <-chan Candle) <-chan Candle {
	outchan := make(chan Candle, 1)

	go func() {
		defer close(outchan)
		for candle := range inputCandleChan {
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

type RunResult struct {
}

// Cerbero is in honor to https://www.backtrader.com/
// that deeply inspired this code
type Cerbero struct {
	Broker              Broker
	Strategy            Strategy
	DataFeed            DataFeed
	TimeAggregationFunc TimeAggregation
}

func (cerbero *Cerbero) Run() (RunResult, error) {
	var wg sync.WaitGroup

	// cerbero consumes from the basefeed and need to fan-out the candles to multiple channels:
	// --> the time aggregator
	// --> the brocker?
	// This is
	baseFeedCloneForTimeAggregation := make(chan Candle, 1000)
	aggregatedFeed := cerbero.TimeAggregationFunc(baseFeedCloneForTimeAggregation)

	// async consume the candles and feed them
	// in our fan-out channels
	wg.Add(1)
	go func() {
		defer close(baseFeedCloneForTimeAggregation)
		defer wg.Done()
		basefeed, err := cerbero.DataFeed.Run()
		if err != nil {
			log.Fatalf("Error consuming base feed -- %v", err)
			return
		}
		log.Println("started base feed consumer routine")
		for tick := range basefeed {
			baseFeedCloneForTimeAggregation <- tick
		}
	}()

	// This guy evaluate the strategy
	wg.Add(1)
	go func() {
		defer wg.Done()
		var candles []Candle
		log.Println("started strategy routine")

		for candle := range aggregatedFeed {
			candles = append(candles, candle)
			cerbero.Strategy.Eval(candles)
		}
	}()

	wg.Wait()
	log.Println("trading done! Besst, Totomz")
	return RunResult{}, nil
}
