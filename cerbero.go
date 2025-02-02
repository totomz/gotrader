package gotrader

import (
	"go.opencensus.io/stats/view"
	"log"
	"os"
	"sync"
	"time"
)

type AggregatedCandle struct {
	Original         Candle
	AggregatedCandle Candle
	IsAggregated     bool
}

type ExecutionResult struct {
	TotalTime       time.Duration `json:"total_time"`
	TotalTimeString string        `json:"total_time_S"`
	InitialCash     float64       `json:"initial_cash"`
	PL              float64       `json:"pl"`
	FinalCash       float64       `json:"final_cash"`
}

var (
	MetricActivePosition = NewMetricWithDefaultViews("broker_position")
	BrockerPosition      = NewMetricWithDefaultViews("broker")

	// Stdout is used to log everything that is not considered an error
	Stdout = log.New(os.Stdout, "", log.Lshortfile|log.Ltime)
	// Stderr log errors that need attention
	Stderr = log.New(os.Stderr, "[ERROR]", log.Lmsgprefix|log.Lshortfile|log.Ltime)
)

// TimeAggregation aggregate the candles from a channel and write the output in a separate channel
type TimeAggregation func(<-chan Candle) <-chan AggregatedCandle

func NoAggregation(inputCandleChan <-chan Candle) <-chan AggregatedCandle {
	outchan := make(chan AggregatedCandle, 1)

	go func() {
		defer close(outchan)
		for candle := range inputCandleChan {
			outchan <- AggregatedCandle{
				Original:         candle,
				AggregatedCandle: candle,
				IsAggregated:     true,
			}
		}
	}()
	return outchan
}

func AggregateBySeconds(sec int) TimeAggregation {
	return func(inputCandleChan <-chan Candle) <-chan AggregatedCandle {
		outchan := make(chan AggregatedCandle, 10000)

		go func() {
			defer close(outchan)

			aggregatedCandle := map[Symbol]Candle{}
			aggregatedCounter := map[Symbol]int{}
			for candle := range inputCandleChan {

				aggregatedCount := aggregatedCounter[candle.Symbol]
				aggregated, existsCandle := aggregatedCandle[candle.Symbol]
				if !existsCandle {
					aggregated = Candle{}
					aggregatedCandle[candle.Symbol] = aggregated
				}

				aggregated = mergeCandles(aggregated, candle)

				outchan <- AggregatedCandle{
					Original:         candle,
					AggregatedCandle: aggregated,
					IsAggregated:     aggregatedCount == sec,
				}

				if aggregatedCount == sec {
					aggregated = Candle{}
					aggregatedCount = 0
				}
				aggregatedCount += 1

				aggregatedCounter[candle.Symbol] = aggregatedCount
				aggregatedCandle[candle.Symbol] = aggregated
			}
		}()
		return outchan
	}
}

// mergeCandles suppose that a is before b.
func mergeCandles(a Candle, b Candle) Candle {
	merged := Candle{Symbol: b.Symbol}

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

// Cerbero is in honor to https://www.backtrader.com/
// that deeply inspired this code
type Cerbero struct {
	Broker              Broker
	Strategy            Strategy
	DataFeed            DataFeed
	TimeAggregationFunc TimeAggregation
	// Stdout              *log.Logger
	// Stderr              *log.Logger
	registeredViews []*view.View
	// Signals             Signal
}

func (cerbero *Cerbero) Run() (ExecutionResult, error) {

	var wg sync.WaitGroup
	start := time.Now()
	execStats := ExecutionResult{
		InitialCash: cerbero.Broker.AvailableCash(),
	}

	// Set default values
	if cerbero.TimeAggregationFunc == nil {
		cerbero.TimeAggregationFunc = NoAggregation
	}

	// cerbero consumes from the basefeed and need to fan-out the candles to multiple channels:
	// --> the time aggregator
	// --> the brocker?
	// This is
	baseFeedCloneForTimeAggregation := make(chan Candle, 1000)
	aggregatedFeed := cerbero.TimeAggregationFunc(baseFeedCloneForTimeAggregation)

	// this routine consume the candles and feed them
	// in the fan-out channel
	wg.Add(1)
	go func() {
		defer close(baseFeedCloneForTimeAggregation)
		defer wg.Done()
		basefeed, err := cerbero.DataFeed.Run()
		if err != nil {
			Stderr.Fatalf("Error consuming base feed -- %v", err)
		}
		Stdout.Println("started base feed consumer routine")

		for tick := range basefeed {
			baseFeedCloneForTimeAggregation <- tick
		}
	}()

	cerbero.Strategy.Initialize(cerbero)

	wg.Add(1)
	go func() {
		defer wg.Done()
		var candles []Candle
		Stdout.Println("started strategy routine")

		for aggregated := range aggregatedFeed {
			ctx := GetNewContextFromCandle(aggregated.AggregatedCandle)
			// notify the broker that it must process all the orders in the queue
			// run it synchronously with the datafeed for backtest.
			// Realtime broker may use this as a "pre-strategy" entry point
			_ = cerbero.Broker.ProcessOrders(aggregated.Original)

			v := cerbero.Broker.AvailableCash()
			pos := cerbero.Broker.GetPositions()

			MCash.Record(ctx, v)
			MStartingCash.Record(ctx, cerbero.Broker.AvailableCash())

			// // cerbero.Signals.Append(aggregated.AggregatedCandle, "cash", v)
			for _, p := range pos {
				c := GetNewContextFromCandle(Candle{Symbol: aggregated.Original.Symbol, Time: aggregated.Original.Time})
				MetricActivePosition.Record(c, float64(p.Size))
				BrockerPosition.Record(c, float64(p.Size))
			}

			// Only orders are processed with the raw candles
			if !aggregated.IsAggregated {
				continue
			}

			// Once orders are processed, we should update the available cash,
			// the broker state and all the fisgnals
			MCandleOpen.Record(ctx, aggregated.AggregatedCandle.Open)
			MCandleHigh.Record(ctx, aggregated.AggregatedCandle.High)
			MCandleClose.Record(ctx, aggregated.AggregatedCandle.Close)
			MCandleLow.Record(ctx, aggregated.AggregatedCandle.Low)
			MCandleVolume.Record(ctx, float64(aggregated.AggregatedCandle.Volume))
			// cerbero.Signals.Append(aggregated.AggregatedCandle, "candle_open", aggregated.AggregatedCandle.Open)
			// cerbero.Signals.Append(aggregated.AggregatedCandle, "candle_high", aggregated.AggregatedCandle.High)
			// cerbero.Signals.Append(aggregated.AggregatedCandle, "candle_low", aggregated.AggregatedCandle.Low)
			// cerbero.Signals.Append(aggregated.AggregatedCandle, "candle_close", aggregated.AggregatedCandle.Close)
			// cerbero.Signals.Append(aggregated.AggregatedCandle, "candle_volume", float64(aggregated.AggregatedCandle.Volume))

			candles = append(candles, aggregated.AggregatedCandle)
			cerbero.Strategy.Eval(candles)

		}
	}()

	wg.Wait()
	cerbero.Strategy.Shutdown()
	cerbero.Broker.Shutdown()

	execStats.TotalTime = time.Now().Sub(start)
	execStats.TotalTimeString = execStats.TotalTime.String()
	execStats.FinalCash = cerbero.Broker.AvailableCash()
	execStats.PL = (execStats.FinalCash/execStats.InitialCash - 1) * 100
	return execStats, nil
}

func Open(candles []Candle) []float64 {
	res := make([]float64, len(candles))
	for i, c := range candles {
		res[i] = c.Open
	}
	return res
}

func Close(candles []Candle) []float64 {
	res := make([]float64, len(candles))
	for i, c := range candles {
		res[i] = c.Close
	}
	return res
}

func High(candles []Candle) []float64 {
	res := make([]float64, len(candles))
	for i, c := range candles {
		res[i] = c.High
	}
	return res
}

func Low(candles []Candle) []float64 {
	res := make([]float64, len(candles))
	for i, c := range candles {
		res[i] = c.Low
	}
	return res
}
