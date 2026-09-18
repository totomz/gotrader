package gotrader

import (
	"bytes"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/exp/slog"
)

func writeTradesFixture(t *testing.T, path string, trades []Trade) {
	t.Helper()

	if err := writeParquetTrades(path, trades); err != nil {
		t.Fatalf("Error writing the trades fixture -- %v", err)
	}
}

func readTradesFixture(t *testing.T, path string, buffer int) []Trade {
	t.Helper()

	out := make(chan Trade, buffer)
	if err := readParquetTrades(path, out); err != nil {
		t.Fatalf("Error reading the parquet trades -- %v", err)
	}
	close(out)

	var trades []Trade
	for trade := range out {
		trades = append(trades, trade)
	}

	return trades
}

func TestReadParquetTrades_RoundTrip(t *testing.T) {
	want := []Trade{
		{Ticker: testSymbol, TS: 1000, ParticipantTS: 999, Seq: 1, Price: 10.5, Size: 100, Exchange: 4, Tape: 3, Conditions: nil, UpdatesLast: true, UpdatesVolume: true, IsRTH: true},
		{Ticker: testSymbol, TS: 1000, ParticipantTS: 1001, Seq: 2, Price: 10.75, Size: 50, Exchange: 11, Tape: 1, Conditions: []int16{12}, UpdatesLast: false, UpdatesVolume: true, IsRTH: false},
		{Ticker: testSymbol, TS: 2000, ParticipantTS: 1999, Seq: 3, Price: 11, Size: 1, Exchange: -1, Tape: 2, Conditions: []int16{1, 37, 255}, UpdatesLast: true, UpdatesVolume: false, IsRTH: true},
		{Ticker: testSymbol, TS: 3000, ParticipantTS: 3000, Seq: 4, Price: 9.25, Size: 3200, Exchange: 8, Tape: 3, Conditions: nil, UpdatesLast: false, UpdatesVolume: false, IsRTH: false},
		{Ticker: testSymbol, TS: 3000, ParticipantTS: 3005, Seq: 5, Price: 9.3, Size: 7, Exchange: 0, Tape: 0, Conditions: []int16{-2, 41}, UpdatesLast: true, UpdatesVolume: true, IsRTH: true},
	}

	path := parquetPathFor(parquetKindTrades, t.TempDir(), testSday, testSymbol)
	writeTradesFixture(t, path, want)

	got := readTradesFixture(t, path, len(want))
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("Unexpected trades (-want +got):\n%s", diff)
	}
}

func TestReadParquetTrades_OutOfOrder(t *testing.T) {
	fixture := []Trade{
		{Ticker: testSymbol, TS: 1000, Seq: 1, Price: 10, Size: 10, UpdatesLast: true, UpdatesVolume: true, IsRTH: true},
		{Ticker: testSymbol, TS: 2000, Seq: 2, Price: 11, Size: 20, UpdatesLast: true, UpdatesVolume: true, IsRTH: true},
		{Ticker: testSymbol, TS: 1500, Seq: 3, Price: 12, Size: 30, UpdatesLast: true, UpdatesVolume: true, IsRTH: true},
		{Ticker: testSymbol, TS: 3000, Seq: 4, Price: 13, Size: 40, UpdatesLast: true, UpdatesVolume: true, IsRTH: true},
		{Ticker: testSymbol, TS: 4000, Seq: 5, Price: 14, Size: 50, UpdatesLast: true, UpdatesVolume: true, IsRTH: true},
	}
	want := []Trade{fixture[0], fixture[1], fixture[3], fixture[4]}

	path := parquetPathFor(parquetKindTrades, t.TempDir(), testSday, testSymbol)
	writeTradesFixture(t, path, fixture)

	var logs bytes.Buffer
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	got := readTradesFixture(t, path, len(fixture))
	slog.SetDefault(previousLogger)

	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("Unexpected trades (-want +got):\n%s", diff)
	}

	if !strings.Contains(logs.String(), "level=ERROR") {
		t.Fatalf("Expected an ERROR log line, got %q", logs.String())
	}
}

func writeQuotesFixture(t *testing.T, path string, quotes []Quote) {
	t.Helper()

	if err := writeParquetQuotes(path, quotes); err != nil {
		t.Fatalf("Error writing the quotes fixture -- %v", err)
	}
}

func readQuotesFixture(t *testing.T, path string, buffer int) []Quote {
	t.Helper()

	out := make(chan Quote, buffer)
	if err := readParquetQuotes(path, out); err != nil {
		t.Fatalf("Error reading the parquet quotes -- %v", err)
	}
	close(out)

	var quotes []Quote
	for quote := range out {
		quotes = append(quotes, quote)
	}

	return quotes
}

func TestReadParquetQuotes_RoundTrip(t *testing.T) {
	want := []Quote{
		{Ticker: testSymbol, TS: 1000, Seq: 1, BidPrice: 10.25, AskPrice: 10.3, BidSize: 100, AskSize: 200, BidExchange: 4, AskExchange: 11, Condition: 1, Indicators: []int16{7}, Tape: 3},
		{Ticker: testSymbol, TS: 1000, Seq: 2, BidPrice: 10.5, AskPrice: 10.4, BidSize: 0, AskSize: 0, BidExchange: -1, AskExchange: 0, Condition: -2, Indicators: nil, Tape: 1},
		{Ticker: testSymbol, TS: 2000, Seq: 3, BidPrice: 10.4, AskPrice: 10.4, BidSize: 5, AskSize: 0, BidExchange: 8, AskExchange: 8, Condition: 0, Indicators: []int16{1, 37, 255}, Tape: 2},
		{Ticker: testSymbol, TS: 3000, Seq: 4, BidPrice: 0, AskPrice: 0, BidSize: 0, AskSize: 0, BidExchange: 0, AskExchange: 0, Condition: 0, Indicators: nil, Tape: 0},
	}

	path := parquetPathFor(parquetKindQuotes, t.TempDir(), testSday, testSymbol)
	writeQuotesFixture(t, path, want)

	got := readQuotesFixture(t, path, len(want))
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("Unexpected quotes (-want +got):\n%s", diff)
	}

	if got[1].IsValid() {
		t.Fatalf("Expected the crossed quote to be invalid, got %v", got[1])
	}
}

func TestReadParquetQuotes_NanosecondsTimestamps(t *testing.T) {
	want := []Quote{
		{Ticker: testSymbol, TS: 1733913000123456789, Seq: 1, BidPrice: 10.25, AskPrice: 10.3, BidSize: 100, AskSize: 200, Tape: 3},
		{Ticker: testSymbol, TS: 1733913000987654321, Seq: 2, BidPrice: 10.26, AskPrice: 10.31, BidSize: 10, AskSize: 20, Tape: 3},
	}

	path := parquetPathFor(parquetKindQuotes, t.TempDir(), testSday, testSymbol)
	writeQuotesFixture(t, path, want)

	got := readQuotesFixture(t, path, len(want))
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("Unexpected quotes (-want +got):\n%s", diff)
	}
}

func TestLessMarketEvent(t *testing.T) {
	tradeSeq2 := Trade{Ticker: Symbol("AAPL"), TS: 10, Seq: 2, Price: 10, Size: 1}
	quoteSeq9 := Quote{Ticker: Symbol("AAPL"), TS: 10, Seq: 9, BidPrice: 9.9, AskPrice: 10.1}
	tradeSeq1 := Trade{Ticker: Symbol("AAPL"), TS: 10, Seq: 1, Price: 11, Size: 2}
	quoteEarly := Quote{Ticker: Symbol("AAPL"), TS: 5, Seq: 4, BidPrice: 9.8, AskPrice: 10.2}
	tradeOther := Trade{Ticker: Symbol("MSFT"), TS: 10, Seq: 1, Price: 20, Size: 3}

	got := []MarketEvent{
		{Trade: &tradeSeq2},
		{Quote: &quoteSeq9},
		{Trade: &tradeSeq1},
		{Quote: &quoteEarly},
		{Trade: &tradeOther},
	}

	want := []MarketEvent{
		{Quote: &quoteEarly},
		{Quote: &quoteSeq9},
		{Trade: &tradeSeq1},
		{Trade: &tradeOther},
		{Trade: &tradeSeq2},
	}

	sort.Slice(got, func(i, j int) bool { return lessMarketEvent(got[i], got[j]) })

	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("Unexpected order (-want +got):\n%s", diff)
	}
}

func TestGenericParquetTrades_Run(t *testing.T) {
	symbolA := Symbol("AAA")
	symbolB := Symbol("BBB")
	folder := t.TempDir()

	tradeA1 := Trade{Ticker: symbolA, TS: 1000, ParticipantTS: 999, Seq: 1, Price: 10, Size: 5, UpdatesLast: true, UpdatesVolume: true, IsRTH: true}
	tradeA2 := Trade{Ticker: symbolA, TS: 2000, ParticipantTS: 1999, Seq: 4, Price: 11, Size: 6, UpdatesLast: true, UpdatesVolume: true, IsRTH: true}
	quoteA1 := Quote{Ticker: symbolA, TS: 1000, Seq: 2, BidPrice: 9.9, AskPrice: 10.1, BidSize: 100, AskSize: 200, Tape: 3}
	quoteA2 := Quote{Ticker: symbolA, TS: 1500, Seq: 3, BidPrice: 9.95, AskPrice: 10.05, BidSize: 10, AskSize: 20, Tape: 3}

	tradeB1 := Trade{Ticker: symbolB, TS: 1000, ParticipantTS: 1000, Seq: 1, Price: 20, Size: 7, UpdatesLast: true, UpdatesVolume: true, IsRTH: true}
	tradeB2 := Trade{Ticker: symbolB, TS: 2500, ParticipantTS: 2500, Seq: 7, Price: 21, Size: 8, UpdatesLast: true, UpdatesVolume: true, IsRTH: true}
	quoteB1 := Quote{Ticker: symbolB, TS: 500, Seq: 1, BidPrice: 19.9, AskPrice: 20.1, BidSize: 30, AskSize: 40, Tape: 1}
	quoteB2 := Quote{Ticker: symbolB, TS: 2000, Seq: 4, BidPrice: 20.9, AskPrice: 21.1, BidSize: 50, AskSize: 60, Tape: 1}

	writeTradesFixture(t, parquetPathFor(parquetKindTrades, folder, testSday, symbolA), []Trade{tradeA1, tradeA2})
	writeQuotesFixture(t, parquetPathFor(parquetKindQuotes, folder, testSday, symbolA), []Quote{quoteA1, quoteA2})
	writeTradesFixture(t, parquetPathFor(parquetKindTrades, folder, testSday, symbolB), []Trade{tradeB1, tradeB2})
	writeQuotesFixture(t, parquetPathFor(parquetKindQuotes, folder, testSday, symbolB), []Quote{quoteB1, quoteB2})

	want := []MarketEvent{
		{Quote: &quoteB1},
		{Quote: &quoteA1},
		{Trade: &tradeA1},
		{Trade: &tradeB1},
		{Quote: &quoteA2},
		{Quote: &quoteB2},
		{Trade: &tradeA2},
		{Trade: &tradeB2},
	}

	var feed TickFeed = &GenericParquetTrades{DataFolder: folder, Day: testSday, Symbols: []Symbol{symbolA, symbolB}}

	stream, err := feed.Run()
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	var got []MarketEvent
	for event := range stream {
		got = append(got, event)
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("Unexpected stream (-want +got):\n%s", diff)
	}

	for i := 1; i < len(got); i++ {
		if got[i].TS() < got[i-1].TS() {
			t.Fatalf("The stream is not sorted by ts: %v after %v", got[i].TS(), got[i-1].TS())
		}
	}
}

func TestGenericParquetTrades_RunMissingQuotes(t *testing.T) {
	symbolA := Symbol("AAA")
	symbolB := Symbol("BBB")
	folder := t.TempDir()

	tradeA1 := Trade{Ticker: symbolA, TS: 1000, Seq: 1, Price: 10, Size: 5, UpdatesLast: true, UpdatesVolume: true, IsRTH: true}
	quoteA1 := Quote{Ticker: symbolA, TS: 900, Seq: 1, BidPrice: 9.9, AskPrice: 10.1, BidSize: 100, AskSize: 200, Tape: 3}
	tradeB1 := Trade{Ticker: symbolB, TS: 1200, Seq: 2, Price: 20, Size: 7, UpdatesLast: true, UpdatesVolume: true, IsRTH: true}

	writeTradesFixture(t, parquetPathFor(parquetKindTrades, folder, testSday, symbolA), []Trade{tradeA1})
	writeQuotesFixture(t, parquetPathFor(parquetKindQuotes, folder, testSday, symbolA), []Quote{quoteA1})
	writeTradesFixture(t, parquetPathFor(parquetKindTrades, folder, testSday, symbolB), []Trade{tradeB1})

	want := []MarketEvent{
		{Quote: &quoteA1},
		{Trade: &tradeA1},
		{Trade: &tradeB1},
	}

	feed := &GenericParquetTrades{DataFolder: folder, Day: testSday, Symbols: []Symbol{symbolA, symbolB}}

	var logs bytes.Buffer
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	stream, err := feed.Run()
	slog.SetDefault(previousLogger)

	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	var got []MarketEvent
	for event := range stream {
		got = append(got, event)
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("Unexpected stream (-want +got):\n%s", diff)
	}

	if !strings.Contains(logs.String(), "level=INFO") || !strings.Contains(logs.String(), string(symbolB)) {
		t.Fatalf("Expected an INFO log line about the missing quotes of %v, got %q", symbolB, logs.String())
	}
}

func TestGenericParquetTrades_RunMissingTrades(t *testing.T) {
	symbolA := Symbol("AAA")
	symbolB := Symbol("BBB")
	folder := t.TempDir()

	tradeA1 := Trade{Ticker: symbolA, TS: 1000, Seq: 1, Price: 10, Size: 5, UpdatesLast: true, UpdatesVolume: true, IsRTH: true}
	quoteA1 := Quote{Ticker: symbolA, TS: 900, Seq: 1, BidPrice: 9.9, AskPrice: 10.1, BidSize: 100, AskSize: 200, Tape: 3}

	writeTradesFixture(t, parquetPathFor(parquetKindTrades, folder, testSday, symbolA), []Trade{tradeA1})
	writeQuotesFixture(t, parquetPathFor(parquetKindQuotes, folder, testSday, symbolA), []Quote{quoteA1})

	feed := &GenericParquetTrades{DataFolder: folder, Day: testSday, Symbols: []Symbol{symbolA, symbolB}}

	goroutines := runtime.NumGoroutine()

	stream, err := feed.Run()
	if err == nil {
		t.Fatalf("Run() error = nil, want an error for the missing trades file of %v", symbolB)
	}

	if stream != nil {
		t.Fatalf("Run() stream = %v, want nil", stream)
	}

	if runtime.NumGoroutine() > goroutines {
		t.Fatalf("Run() leaked %v goroutines", runtime.NumGoroutine()-goroutines)
	}
}
