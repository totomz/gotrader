package massive

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/totomz/gotrader"
)

func TestConfigSymbols(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		tickers []string
		want    []gotrader.Symbol
	}{
		{
			name:    "keeps order and case",
			tickers: []string{"AAPL", "msft", "Tsla"},
			want:    []gotrader.Symbol{"AAPL", "msft", "Tsla"},
		},
		{
			name:    "empty ticker list",
			tickers: nil,
			want:    []gotrader.Symbol{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			config := Config{Tickers: test.tickers}

			if diff := cmp.Diff(test.want, config.Symbols()); diff != "" {
				t.Errorf("Symbols() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
