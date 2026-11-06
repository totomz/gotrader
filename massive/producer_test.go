package massive

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/totomz/gotrader"
	"github.com/twmb/franz-go/pkg/kgo"
)

// unreachableBroker is a port nothing listens on: it is how the write-error
// path is exercised without a cluster.
const unreachableBroker = "127.0.0.1:1"

// cancelledBatchSize is the batch the shutdown test publishes. It is far
// beyond the 10000 records franz-go buffers by default, so the drain loop goes
// through the blocking wait of a full buffer at least twice.
const cancelledBatchSize = 30000

func TestRecordShapeTrade(t *testing.T) {
	trade := gotrader.Trade{Ticker: "aapl", TS: 1, Seq: 2, Price: 10, Size: 5, Exchange: 4, Tape: 1, Conditions: []int16{12}}

	record, err := recordFor(gotrader.MarketEvent{Trade: &trade})
	if err != nil {
		t.Fatalf("recordFor() error = %v", err)
	}

	if record.Topic != "AAPL" {
		t.Errorf("topic = %q, want AAPL", record.Topic)
	}

	if string(record.Key) != "AAPL" {
		t.Errorf("key = %q, want AAPL", record.Key)
	}

	if got := headerValue(record, headerKind); got != kindTrade {
		t.Errorf("header %s = %q, want %q", headerKind, got, kindTrade)
	}

	var decoded gotrader.Trade
	if err = json.Unmarshal(record.Value, &decoded); err != nil {
		t.Fatalf("can not decode the record value: %v", err)
	}

	if diff := cmp.Diff(trade, decoded); diff != "" {
		t.Errorf("the trade did not round trip (-want +got):\n%s", diff)
	}
}

func TestRecordShapeQuote(t *testing.T) {
	quote := gotrader.Quote{Ticker: "aapl", TS: 1, Seq: 2, BidPrice: 9, AskPrice: 10, BidSize: 1, AskSize: 2, BidExchange: 3, AskExchange: 4, Condition: 1, Indicators: []int16{7}, Tape: 2}

	record, err := recordFor(gotrader.MarketEvent{Quote: &quote})
	if err != nil {
		t.Fatalf("recordFor() error = %v", err)
	}

	if record.Topic != "AAPL" {
		t.Errorf("topic = %q, want AAPL", record.Topic)
	}

	if string(record.Key) != "AAPL" {
		t.Errorf("key = %q, want AAPL", record.Key)
	}

	if got := headerValue(record, headerKind); got != kindQuote {
		t.Errorf("header %s = %q, want %q", headerKind, got, kindQuote)
	}

	var decoded gotrader.Quote
	if err = json.Unmarshal(record.Value, &decoded); err != nil {
		t.Fatalf("can not decode the record value: %v", err)
	}

	if diff := cmp.Diff(quote, decoded); diff != "" {
		t.Errorf("the quote did not round trip (-want +got):\n%s", diff)
	}
}

func TestRecordForUnpublishableEvents(t *testing.T) {
	tests := map[string]gotrader.MarketEvent{
		"an empty event":           {},
		"a trade without ticker":   {Trade: &gotrader.Trade{TS: 1}},
		"a quote without a ticker": {Quote: &gotrader.Quote{TS: 1}},
	}

	for name, event := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := recordFor(event); err == nil {
				t.Errorf("recordFor(%s) error = nil, want an error", name)
			}
		})
	}
}

// TestPublishWriteErrors publishes to a port nothing listens on, with an
// already cancelled context: the shutdown of this pipeline is exactly that.
// The records can only fail, and they must fail on their own delivery timeout
// rather than on the cancelled context, while the loop keeps draining.
func TestPublishWriteErrors(t *testing.T) {
	reader := installReader(t)

	client, err := kgo.NewClient(
		kgo.SeedBrokers(unreachableBroker),
		kgo.RecordDeliveryTimeout(time.Second),
	)
	if err != nil {
		t.Fatalf("can not build the redpanda client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	trade := gotrader.Trade{Ticker: "aapl", TS: 1, Price: 10, Size: 5}
	quote := gotrader.Quote{Ticker: "aapl", TS: 2, BidPrice: 9, AskPrice: 10, BidSize: 1, AskSize: 1}

	events := make(chan gotrader.MarketEvent, 3)
	events <- gotrader.MarketEvent{Trade: &trade}
	events <- gotrader.MarketEvent{}
	events <- gotrader.MarketEvent{Quote: &quote}
	close(events)

	m, err := newMetrics()
	if err != nil {
		t.Fatalf("newMetrics() error = %v", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- publish(ctx, client, events, m)
	}()

	select {
	case err = <-done:
	case <-time.After(testTimeout):
		t.Fatal("publish() did not return with an unreachable broker")
	}

	var writeErrors WriteErrors
	if !errors.As(err, &writeErrors) {
		t.Fatalf("publish() error = %v, want a WriteErrors", err)
	}

	if writeErrors.Sent != 2 {
		t.Errorf("sent = %d, want 2: the unpublishable event is skipped, the other two are produced", writeErrors.Sent)
	}

	if writeErrors.Failed != 2 {
		t.Errorf("failed = %d, want 2: no record can reach an unreachable broker", writeErrors.Failed)
	}

	if len(events) != 0 {
		t.Errorf("publish() left %d events in the channel, want 0", len(events))
	}

	if got := collectCounter(context.Background(), t, reader, "massive.write.errors"); got != writeErrors.Failed {
		t.Errorf("the write error counter is %d, want %d: it must move with the returned WriteErrors", got, writeErrors.Failed)
	}
}

func TestPublishToRedpanda(t *testing.T) {
	broker := os.Getenv("REDPANDA_BROKERS")
	if broker == "" {
		t.Skip("REDPANDA_BROKERS is not set, skipping the redpanda write test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ticker := gotrader.Symbol(fmt.Sprintf("test%d", time.Now().UnixNano()))
	topic := strings.ToUpper(string(ticker))
	trade := gotrader.Trade{Ticker: ticker, TS: time.Now().UnixMilli(), Seq: 1, Price: 1, Size: 1}
	quote := gotrader.Quote{Ticker: ticker, TS: time.Now().UnixMilli(), Seq: 2, BidPrice: 1, AskPrice: 2, BidSize: 1, AskSize: 1}

	events := make(chan gotrader.MarketEvent, 2)
	events <- gotrader.MarketEvent{Trade: &trade}
	events <- gotrader.MarketEvent{Quote: &quote}
	close(events)

	if err := Publish(ctx, Config{Kafka: broker}, events); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	records := readBack(ctx, t, broker, topic, 2)

	var decodedTrade gotrader.Trade
	if err := json.Unmarshal(records[0].Value, &decodedTrade); err != nil {
		t.Fatalf("can not decode the trade record: %v", err)
	}

	if diff := cmp.Diff(trade, decodedTrade); diff != "" {
		t.Errorf("the trade did not round trip (-want +got):\n%s", diff)
	}

	var decodedQuote gotrader.Quote
	if err := json.Unmarshal(records[1].Value, &decodedQuote); err != nil {
		t.Fatalf("can not decode the quote record: %v", err)
	}

	if diff := cmp.Diff(quote, decodedQuote); diff != "" {
		t.Errorf("the quote did not round trip (-want +got):\n%s", diff)
	}
}

// TestPublishToRedpandaAfterCancel is the shutdown of the pipeline: the
// context is already cancelled and the handoff channel still holds a batch
// bigger than the client buffer. Every one of those events must land on the
// cluster, in order, instead of being failed by the dead context.
func TestPublishToRedpandaAfterCancel(t *testing.T) {
	broker := os.Getenv("REDPANDA_BROKERS")
	if broker == "" {
		t.Skip("REDPANDA_BROKERS is not set, skipping the redpanda write test")
	}

	ticker := gotrader.Symbol(fmt.Sprintf("test%d", time.Now().UnixNano()))
	topic := strings.ToUpper(string(ticker))

	events := make(chan gotrader.MarketEvent, cancelledBatchSize)
	for i := range cancelledBatchSize {
		trade := gotrader.Trade{Ticker: ticker, TS: int64(i), Seq: int64(i), Price: 1, Size: 1}
		events <- gotrader.MarketEvent{Trade: &trade}
	}
	close(events)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := Publish(ctx, Config{Kafka: broker}, events); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	readCtx, readCancel := context.WithTimeout(context.Background(), time.Minute)
	defer readCancel()

	records := readBack(readCtx, t, broker, topic, cancelledBatchSize)
	if len(records) != cancelledBatchSize {
		t.Fatalf("read back %d records, want %d", len(records), cancelledBatchSize)
	}

	for i, record := range records {
		var decoded gotrader.Trade
		if err := json.Unmarshal(record.Value, &decoded); err != nil {
			t.Fatalf("can not decode the record %d: %v", i, err)
		}

		if decoded.Seq != int64(i) {
			t.Fatalf("record %d has seq %d: the batch is not in order", i, decoded.Seq)
		}
	}
}

// readBack consumes want records from the beginning of a topic, so the e2e
// tests can assert what actually landed on the cluster.
func readBack(ctx context.Context, t *testing.T, broker, topic string, want int) []*kgo.Record {
	t.Helper()

	client, err := kgo.NewClient(
		kgo.SeedBrokers(broker),
		kgo.ConsumeTopics(topic),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
	)
	if err != nil {
		t.Fatalf("can not build the redpanda consumer: %v", err)
	}
	defer client.Close()

	var records []*kgo.Record
	for len(records) < want {
		fetches := client.PollFetches(ctx)
		if err = fetches.Err(); err != nil {
			t.Fatalf("can not read the topic %s: %v", topic, err)
		}
		records = append(records, fetches.Records()...)
	}

	for _, record := range records {
		if string(record.Key) != topic {
			t.Errorf("key = %q, want %q", record.Key, topic)
		}
		if kind := headerValue(record, headerKind); kind != kindTrade && kind != kindQuote {
			t.Errorf("header %s = %q, want %q or %q", headerKind, kind, kindTrade, kindQuote)
		}
	}

	return records
}

// headerValue returns the value of a record header, or an empty string when
// the header is not there.
func headerValue(record *kgo.Record, key string) string {
	for _, header := range record.Headers {
		if header.Key == key {
			return string(header.Value)
		}
	}
	return ""
}
