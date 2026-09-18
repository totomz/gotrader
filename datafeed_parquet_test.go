package gotrader

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/parquet-go/parquet-go"
	"golang.org/x/exp/slog"
)

func writeTradesFixture(t *testing.T, path string, trades []Trade) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("Error creating the fixture folder -- %v", err)
	}

	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("Error creating the fixture file -- %v", err)
	}

	rows := make([]parquetTradeRow, len(trades))
	for i, trade := range trades {
		rows[i] = tradeToParquetRow(trade)
	}

	writer := parquet.NewGenericWriter[parquetTradeRow](file)
	if _, err = writer.Write(rows); err != nil {
		t.Fatalf("Error writing the fixture rows -- %v", err)
	}

	if err = writer.Close(); err != nil {
		t.Fatalf("Error closing the fixture writer -- %v", err)
	}

	if err = file.Close(); err != nil {
		t.Fatalf("Error closing the fixture file -- %v", err)
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

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("Error creating the fixture folder -- %v", err)
	}

	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("Error creating the fixture file -- %v", err)
	}

	rows := make([]parquetQuoteRow, len(quotes))
	for i, quote := range quotes {
		rows[i] = quoteToParquetRow(quote)
	}

	writer := parquet.NewGenericWriter[parquetQuoteRow](file)
	if _, err = writer.Write(rows); err != nil {
		t.Fatalf("Error writing the fixture rows -- %v", err)
	}

	if err = writer.Close(); err != nil {
		t.Fatalf("Error closing the fixture writer -- %v", err)
	}

	if err = file.Close(); err != nil {
		t.Fatalf("Error closing the fixture file -- %v", err)
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
