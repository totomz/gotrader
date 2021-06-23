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
