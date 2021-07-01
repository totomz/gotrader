package gotrader

import (
	"github.com/google/go-cmp/cmp"
	"testing"
	"time"
)

func TestTimeAggregationNone(t *testing.T) {
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
		got = append(got, candle)
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("NoAggregation() mismatch (-want +got):\n%s", diff)
	}

}

func TestTimeAggregation_15Sec(t *testing.T) {
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
	for candle := range outchan {
		candles = append(candles, candle)
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
	InitializeImpl func(broker Broker)
}

func (me *testMockStrategy) Eval(candles []Candle) {
	if me.EvalImpl != nil {
		me.EvalImpl(candles)
	}
}

func (me *testMockStrategy) Initialize(broker Broker) {
	if me.InitializeImpl != nil {
		me.InitializeImpl(broker)
	}
}

func TestStrategyReadsCandles(t *testing.T) {

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
		Broker:              &BacktestBrocker{InitialCashUSD: 30000},
		TimeAggregationFunc: NoAggregation,
		DataFeed: &IBZippedCSV{
			DataFolder: testFolder,
			Symbol:     testSymbol,
			Sday:       testSday,
		},
	}

	_, err := service.Run()

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

func TestSimpleOrderExecution(t *testing.T) {

	var _broker Broker

	buySell := testMockStrategy{
		InitializeImpl: func(broker Broker) {
			_broker = broker
		},
		EvalImpl: func(candles []Candle) {
			latest := candles[len(candles)-1]

			if latest.Time.Equal(time.Date(2021, 1, 11, 18, 23, 30, 0, time.Local)) {
				// Expect to buy the second after this inst @ 262.23
				println(_broker)
				println("BUY")
			}

			if latest.Time.Equal(time.Date(2021, 1, 11, 18, 36, 45, 0, time.Local)) {
				// Expect to sell the second after this inst @ 262.86
				println("SELL")
			}
		},
	}
	service := Cerbero{
		Strategy:            &buySell,
		Broker:              &BacktestBrocker{InitialCashUSD: 30000},
		TimeAggregationFunc: AggregateBySeconds(15),
		DataFeed: &IBZippedCSV{
			DataFolder: testFolder,
			Symbol:     testSymbol,
			Sday:       testSday,
		},
	}

	results, err := service.Run()
	if err != nil {
		t.Fatal(err)
	}

	if results.PL != 0.6 {
		t.Fatalf("expected a P&L of $0.6, got %v", results.PL)
	}

	if results.Commissions != 0 {
		t.Fatalf("expected 0 commissions, got %v", results.Commissions)
	}
}
