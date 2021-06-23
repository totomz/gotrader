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
