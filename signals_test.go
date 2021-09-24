package gotrader

import (
	"github.com/google/go-cmp/cmp"
	"sort"
	"testing"
	"time"
)

func TestAddGetSignals(t *testing.T) {
	signals := Signal{}

	now := time.Now()

	for i := 1; i < 100; i++ {
		now = now.Add(1 * time.Second)
		candle := Candle{Time: now}
		values := make([]float64, i)

		for j := 0; j < i; j++ {
			values[j] = float64(i)
		}

		signals.AppendFloat(candle, "keya", values[len(values)-1])
		signals.AppendFloat(candle, "keyb", values[len(values)-1])
	}

	// Candles, trades, cash are always added to the signals
	wantedKeys := []string{"keya", "keyb"}
	gotKeys := signals.Keys()

	sort.Strings(wantedKeys)
	sort.Strings(gotKeys)

	if diff := cmp.Diff(wantedKeys, gotKeys); diff != "" {
		t.Errorf("Key() mismatch (-want +got):\n%s", diff)
	}

	vals, _ := signals.Get("keya")
	bomber := vals.(TimeSerieFloat)
	if len(bomber.Y) != len(bomber.Y) {
		t.Error("X and Y should have the same length")
	}
	for i, v := range bomber.Y {
		if i+1 != int(v) {
			t.Errorf("expecting %v got %v", i, v)
		}
	}

	jsonData, err := signals.ToJson()
	if err != nil {
		t.Error(err)
	}
	println(string(jsonData))

	/*
		[{
			'timestamp': '2013-10-04 22:23:00',
			<serieName>: <serieValue>
		}]
	*/
}
