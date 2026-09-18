package gotrader

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestSliceTickFeed_Run(t *testing.T) {
	t.Parallel()

	quote := Quote{Ticker: testSymbol, TS: 1700000000000, Seq: 1, BidPrice: 10, AskPrice: 10.5, BidSize: 100, AskSize: 200}
	trade := Trade{Ticker: testSymbol, TS: 1700000000000, Seq: 2, Price: 10.5, Size: 100, UpdatesLast: true, UpdatesVolume: true}
	other := Trade{Ticker: Symbol("AAPL"), TS: 1700000000500, Seq: 3, Price: 20, Size: 50, UpdatesLast: true}

	want := []MarketEvent{
		{Quote: &quote},
		{Trade: &trade},
		{Trade: &other},
	}

	var feed TickFeed = &SliceTickFeed{Events: want}

	stream, err := feed.Run()
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	var got []MarketEvent
	for event := range stream {
		got = append(got, event)
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Run() mismatch (-want +got):\n%s", diff)
	}
}

func TestSliceTickFeed_RunEmpty(t *testing.T) {
	t.Parallel()

	var feed TickFeed = &SliceTickFeed{}

	stream, err := feed.Run()
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	got := 0
	for range stream {
		got++
	}

	if got != 0 {
		t.Fatalf("Run() delivered %v events, want 0", got)
	}
}
