package gotrader

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestCandleBuilder_Build(t *testing.T) {
	t.Parallel()

	base := time.Date(2023, 11, 14, 14, 30, 0, 0, time.UTC).UnixMilli()
	slotTime := func(offset int64) time.Time { return time.UnixMilli(base + offset).UTC() }

	tests := []struct {
		name   string
		trades []Trade
		want   []Candle
	}{
		{
			name: "carry forward between trades",
			trades: []Trade{
				{Ticker: testSymbol, TS: base + 100, Price: 10, Size: 100, UpdatesLast: true, UpdatesVolume: true},
				{Ticker: testSymbol, TS: base + 3900, Price: 10.5, Size: 200, UpdatesLast: true, UpdatesVolume: true},
				{Ticker: testSymbol, TS: base + 7000, Price: 11, Size: 300, UpdatesLast: true, UpdatesVolume: true},
				{Ticker: testSymbol, TS: base + 21000, Price: 9.5, Size: 400, UpdatesLast: true, UpdatesVolume: true},
			},
			want: []Candle{
				{Open: 10, High: 10.5, Low: 10, Close: 10.5, Volume: 300, Symbol: testSymbol, Time: slotTime(0)},
				{Open: 11, High: 11, Low: 11, Close: 11, Volume: 300, Symbol: testSymbol, Time: slotTime(5000)},
				{Open: 11, High: 11, Low: 11, Close: 11, Volume: 0, Symbol: testSymbol, Time: slotTime(10000)},
				{Open: 11, High: 11, Low: 11, Close: 11, Volume: 0, Symbol: testSymbol, Time: slotTime(15000)},
				{Open: 9.5, High: 9.5, Low: 9.5, Close: 9.5, Volume: 400, Symbol: testSymbol, Time: slotTime(20000)},
			},
		},
		{
			name: "volume only and ignored trades",
			trades: []Trade{
				{Ticker: testSymbol, TS: base + 100, Price: 10, Size: 100, UpdatesLast: true, UpdatesVolume: true},
				{Ticker: testSymbol, TS: base + 200, Price: 99, Size: 50, UpdatesLast: false, UpdatesVolume: true},
				{Ticker: testSymbol, TS: base + 300, Price: 77, Size: 999, UpdatesLast: false, UpdatesVolume: false},
			},
			want: []Candle{
				{Open: 10, High: 10, Low: 10, Close: 10, Volume: 150, Symbol: testSymbol, Time: slotTime(0)},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			builder := NewCandleBuilder(5000)

			var got []Candle
			for _, trade := range tt.trades {
				got = append(got, builder.Push(trade)...)
			}
			got = append(got, builder.Flush()...)

			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("candles mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
