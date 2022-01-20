package gotrader

import (
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

const (
	SIGNAL_CASH        = "cash"
	SIGNAL_TRADES_BUY  = "trades_buy"
	SIGNAL_TRADES_SELL = "trades_sell"
	SIGNAL_CANDLES     = "candles"
)

var ORDERTYPE_TO_SIGNALE = map[OrderType]string{
	OrderBuy:  SIGNAL_TRADES_BUY,
	OrderSell: SIGNAL_TRADES_SELL,
}

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
			i := 0
			aggregated := Candle{}
			for candle := range inputCandleChan {
				aggregated = mergeCandles(aggregated, candle)

				outchan <- AggregatedCandle{
					Original:         candle,
					AggregatedCandle: aggregated,
					IsAggregated:     i == sec,
				}

				if i == sec {
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
	Stdout              *log.Logger
	Stderr              *log.Logger

	signals Signal
}

func (cerbero *Cerbero) Run() (ExecutionResult, error) {

	if cerbero.Stdout == nil {
		cerbero.Stdout = log.New(os.Stdout, "", log.Lshortfile|log.Ltime)
	}
	if cerbero.Stderr == nil {
		cerbero.Stdout = log.New(os.Stdout, "", log.Lshortfile|log.Ltime)
	}

	var wg sync.WaitGroup
	start := time.Now()
	stats := ExecutionResult{
		InitialCash: cerbero.Broker.AvailableCash(),
	}

	// Set default values
	cerbero.signals = Signal{values: map[string]interface{}{}}
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
			cerbero.Stderr.Fatalf("Error consuming base feed -- %v", err)
			return
		}
		cerbero.Stdout.Println("started base feed consumer routine")
		for tick := range basefeed {
			baseFeedCloneForTimeAggregation <- tick
		}
	}()

	cerbero.Strategy.Initialize(cerbero)

	wg.Add(1)
	go func() {
		defer wg.Done()
		var candles []Candle
		cerbero.Stdout.Println("started strategy routine")

		for aggregated := range aggregatedFeed {

			// notify the broker that it must process all the orders in the queue
			// run it synchronously with the datafeed for backtest.
			// Realtime broker may use this as a "pre-strategy" entry point
			ordersExecuted := cerbero.Broker.ProcessOrders(aggregated.Original)
			for _, order := range ordersExecuted {
				cerbero.signals.AppendFloat(aggregated.AggregatedCandle, ORDERTYPE_TO_SIGNALE[order.Type], order.AvgFilledPrice)
			}

			// Only orders are processed with the raw candles
			if !aggregated.IsAggregated {
				continue
			}

			// Once orders are processed, we should update the available cash,
			// the broker state and all the fisgnals
			v := cerbero.Broker.AvailableCash()
			cerbero.signals.AppendFloat(aggregated.AggregatedCandle, SIGNAL_CASH, v)
			cerbero.signals.AppendCandle(aggregated.AggregatedCandle, SIGNAL_CANDLES, aggregated.AggregatedCandle)

			candles = append(candles, aggregated.AggregatedCandle)
			cerbero.Strategy.Eval(candles)

		}
	}()

	wg.Wait()
	cerbero.Broker.Shutdown()

	stats.TotalTime = time.Now().Sub(start)
	stats.TotalTimeString = stats.TotalTime.String()
	stats.FinalCash = cerbero.Broker.AvailableCash()
	stats.PL = (stats.FinalCash/stats.InitialCash - 1) * 100
	return stats, nil
}

func (cerbero *Cerbero) Signals() *Signal {
	return &cerbero.signals
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
