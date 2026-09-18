package gotrader

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestQuote_Validity(t *testing.T) {
	tests := []struct {
		name          string
		quote         Quote
		wantValid     bool
		wantMid       float64
		wantSpread    float64
		wantSpreadRel float64
	}{
		{
			name:          "bid not positive",
			quote:         Quote{Ticker: testSymbol, BidPrice: 0, AskPrice: 10.5, BidSize: 100, AskSize: 100},
			wantValid:     false,
			wantMid:       5.25,
			wantSpread:    10.5,
			wantSpreadRel: 2,
		},
		{
			name:          "ask not positive",
			quote:         Quote{Ticker: testSymbol, BidPrice: 10, AskPrice: 0, BidSize: 100, AskSize: 100},
			wantValid:     false,
			wantMid:       5,
			wantSpread:    -10,
			wantSpreadRel: -2,
		},
		{
			name:          "crossed",
			quote:         Quote{Ticker: testSymbol, BidPrice: 10.5, AskPrice: 10, BidSize: 100, AskSize: 100},
			wantValid:     false,
			wantMid:       10.25,
			wantSpread:    -0.5,
			wantSpreadRel: -0.5 / 10.25,
		},
		{
			name:          "both sizes zero",
			quote:         Quote{Ticker: testSymbol, BidPrice: 10, AskPrice: 10.5, BidSize: 0, AskSize: 0},
			wantValid:     false,
			wantMid:       10.25,
			wantSpread:    0.5,
			wantSpreadRel: 0.5 / 10.25,
		},
		{
			name:          "locked",
			quote:         Quote{Ticker: testSymbol, BidPrice: 10, AskPrice: 10, BidSize: 100, AskSize: 200},
			wantValid:     true,
			wantMid:       10,
			wantSpread:    0,
			wantSpreadRel: 0,
		},
		{
			name:          "normal",
			quote:         Quote{Ticker: testSymbol, BidPrice: 10, AskPrice: 10.5, BidSize: 100, AskSize: 200},
			wantValid:     true,
			wantMid:       10.25,
			wantSpread:    0.5,
			wantSpreadRel: 0.5 / 10.25,
		},
	}

	approx := cmpopts.EquateApprox(0, 1e-12)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.quote.IsValid(); got != tt.wantValid {
				t.Fatalf("IsValid() = %v, want %v", got, tt.wantValid)
			}

			if diff := cmp.Diff(tt.wantMid, tt.quote.Mid(), approx); diff != "" {
				t.Errorf("Mid() mismatch (-want +got):\n%s", diff)
			}

			if diff := cmp.Diff(tt.wantSpread, tt.quote.Spread(), approx); diff != "" {
				t.Errorf("Spread() mismatch (-want +got):\n%s", diff)
			}

			if diff := cmp.Diff(tt.wantSpreadRel, tt.quote.SpreadRel(), approx); diff != "" {
				t.Errorf("SpreadRel() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestQuote_SpreadRelZeroMid(t *testing.T) {
	quote := Quote{Ticker: testSymbol, BidPrice: -1, AskPrice: 1, BidSize: 100, AskSize: 100}

	if got := quote.SpreadRel(); got != 0 {
		t.Fatalf("SpreadRel() = %v, want 0", got)
	}
}

func TestTrade_Time(t *testing.T) {
	trade := Trade{Ticker: testSymbol, TS: 1700000000123}

	got := trade.Time()
	want := time.Date(2023, 11, 14, 22, 13, 20, 123000000, time.UTC)

	if !got.Equal(want) {
		t.Fatalf("Time() = %v, want %v", got, want)
	}

	if got.Location() != time.UTC {
		t.Fatalf("Time() location = %v, want UTC", got.Location())
	}

	if got.Nanosecond() != 123000000 {
		t.Fatalf("Time() nanosecond = %v, want 123000000", got.Nanosecond())
	}
}
