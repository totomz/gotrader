package gotrader

import (
	"github.com/google/go-cmp/cmp"
	"testing"
	"time"
)

func TestTimeAggregationNone(t *testing.T) {
	t.Parallel()
	now := time.Date(2021, 6, 23, 15, 30, 00, 00, time.Local)
	want := []Candle{
		{Open: 1, High: 1, Close: 1, Low: 1, Volume: 1, Time: now},
		{Open: 2, High: 2, Close: 2, Low: 2, Volume: 2, Time: now.Add(1 * time.Second)},
		{Open: 3, High: 3, Close: 3, Low: 3, Volume: 3, Time: now.Add(2 * time.Second)},
		{Open: 4, High: 4, Close: 4, Low: 4, Volume: 4, Time: now.Add(3 * time.Second)},
		{Open: 5, High: 5, Close: 5, Low: 5, Volume: 5, Time: now.Add(4 * time.Second)},
		{Open: 6, High: 6, Close: 6, Low: 6, Volume: 6, Time: now.Add(5 * time.Second)},
	}
	inChannel := make(chan Candle, 1000)
	outChannel := NoAggregation(inChannel)

	for _, c := range want {
		inChannel <- c
	}
	close(inChannel)

	var got []Candle
	for candle := range outChannel {
		got = append(got, candle.AggregatedCandle)
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("NoAggregation() mismatch (-want +got):\n%s", diff)
	}

}

func TestTimeAggregation_15Sec(t *testing.T) {
	t.Parallel()

	reader := IBZippedCSV{
		DataFolder: testFolder,
		Sday:       testSday,
		Symbol:     testSymbol,
	}

	channel, err := reader.Run()
	if err != nil {
		t.Fatal(err)
	}

	outchan := AggregateBySeconds(15)(channel)

	var candles []Candle
	for aggregated := range outchan {
		if aggregated.IsAggregated {
			candles = append(candles, aggregated.AggregatedCandle)
		}
	}
	if len(candles) == 0 {
		t.Fatalf("outchan closed?")
	}

	if candles[0].Time.Second() != 15 {
		t.Errorf("Expected 00:15 for the first candle, got %v", candles[0].Time.Second())
	}
	if candles[1].Time.Second() != 30 {
		t.Errorf("Expected 00:30 for the first candle, got %v", candles[1].Time.Second())
	}

	want := Candle{
		Open:   260.02,
		High:   261.2,
		Close:  260.81,
		Low:    260,
		Volume: 4154,
		Time:   time.Date(2021, 1, 11, 15, 30, 15, 0, time.Local),
	}

	got := candles[0]
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("AggregateBySeconds() mismatch (-want +got):\n%s", diff)
	}
}

type testMockStrategy struct {
	EvalImpl       func(candles []Candle)
	InitializeImpl func(cerbero *Cerbero)
}

func (s *testMockStrategy) Eval(candles []Candle) {
	if s.EvalImpl != nil {
		s.EvalImpl(candles)
	}
}

func (s *testMockStrategy) Initialize(broker *Cerbero) {
	if s.InitializeImpl != nil {
		s.InitializeImpl(broker)
	}
}

func TestStrategyReadsCandles(t *testing.T) {
	t.Parallel()

	countEval := 0
	strategy := testMockStrategy{
		EvalImpl: func(candles []Candle) {
			countEval = countEval + 1 // Just signal that we've been invoked
			if len(candles) != countEval {
				t.Errorf("mmmhhhh")
			}

			// We respect the array: The latest is the newest!
			// candles[0].Time ==> 15:00:00
			// candles[1].Time ==> 15:00:05
			// candles[2].Time ==> 15:00:10
			previous := candles[0]
			for i, this := range candles {
				if i == 0 {
					continue
				}

				if this.Time.Before(previous.Time) {
					t.Errorf("Invalid candle order!")
				}
				previous = this
			}
		},
	}

	service := Cerbero{
		Strategy:            &strategy,
		Broker:              NewBacktestBrocker(30000),
		TimeAggregationFunc: NoAggregation,
		DataFeed: &IBZippedCSV{
			DataFolder: testFolder,
			Symbol:     testSymbol,
			Sday:       testSday,
		},
	}

	err := service.Run()

	// How many lines? The csv has 1 line for each second.
	// How many seconds in the time interval?
	a := time.Date(2021, 6, 15, 15, 30, 00, 00, time.Local)
	b := time.Date(2021, 6, 15, 21, 59, 59, 00, time.Local)
	rows := b.Sub(a).Seconds() + 1 // +1 because seconds starts at 0, line count at 1
	if countEval != int(rows) {
		t.Fatalf("The strategy should have been evaluated 25200 times, but was %v", countEval)
	}

	if err != nil {
		t.Fatal(err)
	}
}

func TestOrderExecutionAfter1sec(t *testing.T) {
	t.Parallel()
	var _broker Broker
	var _orderID string
	var err error

	buySell := testMockStrategy{
		InitializeImpl: func(cerbero *Cerbero) {
			_broker = cerbero.Broker
		},
		EvalImpl: func(candles []Candle) {
			latest := candles[len(candles)-1]

			if latest.Time.Equal(time.Date(2021, 1, 11, 18, 23, 30, 0, time.Local)) {
				// Expect to buy the second after this inst @ 262.23
				_orderID, err = _broker.SubmitOrder(Order{
					Id:     RandUid(),
					Size:   1,
					Symbol: "FB",
					Type:   OrderBuy,
				})
				if err != nil {
					t.Errorf("error buy order -- %v", err)
				}
			}

			// Expect the order to get fullfilled 1 second after being submitted
			// The strategy runs every 15 seconds, but the order are processed at 1s resolution
			if latest.Time.Equal(time.Date(2021, 1, 11, 18, 23, 45, 0, time.Local)) {
				order, err := _broker.GetOrderByID(_orderID)
				if err != nil {
					t.Errorf("error getting order status -- %v", err)
				}
				if order.Status != OrderStatusFullFilled {
					t.Errorf("Expected testorder to be OrderStatusAccepted, was %v", order.Status)
				}
				if order.SizeFilled != 1 {
					t.Errorf("Expected testorder size filled to be 1, was %v", order.SizeFilled)
				}

				position, found := _broker.GetPosition(order.Symbol)
				if !found {
					t.Errorf("open position not found!")
				}

				if !almostEqual(position.AvgPrice, 262.23) {
					t.Errorf("Expected testorder avg filed price to be 262.86, was %v", position.AvgPrice)
				}

			}

			if latest.Time.Equal(time.Date(2021, 1, 11, 18, 36, 45, 0, time.Local)) {
				// Expect to sell the second after this inst @ 262.86
				_, err = _broker.SubmitOrder(Order{
					Id:     RandUid(),
					Size:   1,
					Symbol: "FB",
					Type:   OrderSell,
				})
				if err != nil {
					t.Errorf("error buy order -- %v", err)
				}
			}
		},
	}
	service := Cerbero{
		Strategy:            &buySell,
		Broker:              NewBacktestBrocker(1000),
		TimeAggregationFunc: AggregateBySeconds(15),
		DataFeed: &IBZippedCSV{
			DataFolder: testFolder,
			Symbol:     testSymbol,
			Sday:       testSday,
		},
	}

	err = service.Run()
	if err != nil {
		t.Fatal(err)
	}

	// At the end, I should have a profit os -1@262.86 +1@262.23 = $0.63
	if _broker.AvailableCash() != 1000.63 {
		t.Fatalf("final cahs does not match, got %v", _broker.AvailableCash())
	}

}
