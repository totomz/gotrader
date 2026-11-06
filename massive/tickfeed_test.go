package massive

//
// import (
// 	"context"
// 	"fmt"
// 	"os"
// 	"testing"
// 	"time"
//
// 	"github.com/google/go-cmp/cmp"
// 	"github.com/totomz/gotrader"
// 	"github.com/twmb/franz-go/pkg/kgo"
// )
//
// // sentinelSeq marks the warm-up trades the e2e tests publish until the feed
// // answers. A consumer that starts at the end of a topic is live only once the
// // brokers have answered its first fetch, and nothing in the client API says
// // when that happened: the tests publish until one of their own records comes
// // back, and drop every trade carrying this sequence number afterwards.
// const sentinelSeq = -1
//
// // warmupInterval is how often a sentinel is published while waiting for the
// // feed to become live.
// const warmupInterval = 200 * time.Millisecond
//
// // feedTimeout bounds every e2e test of the feed, so a broker that answers
// // nothing fails the test instead of hanging the suite.
// const feedTimeout = 60 * time.Second
//
// func TestEventFromRecord(t *testing.T) {
// 	trade := gotrader.Trade{Ticker: "aapl", TS: 1, Seq: 2, Price: 10, Size: 5, Exchange: 4, Tape: 1, Conditions: []int16{12}}
// 	quote := gotrader.Quote{Ticker: "aapl", TS: 3, Seq: 4, BidPrice: 9, AskPrice: 10, BidSize: 1, AskSize: 2, BidExchange: 3, AskExchange: 4, Condition: 1, Indicators: []int16{7}, Tape: 2}
//
// 	tests := map[string]gotrader.MarketEvent{
// 		"a trade": {Trade: &trade},
// 		"a quote": {Quote: &quote},
// 	}
//
// 	for name, want := range tests {
// 		t.Run(name, func(t *testing.T) {
// 			record, err := recordFor(want)
// 			if err != nil {
// 				t.Fatalf("recordFor() error = %v", err)
// 			}
//
// 			got, err := eventFromRecord(record)
// 			if err != nil {
// 				t.Fatalf("eventFromRecord() error = %v", err)
// 			}
//
// 			if diff := cmp.Diff(want, got); diff != "" {
// 				t.Errorf("%s did not round trip (-want +got):\n%s", name, diff)
// 			}
// 		})
// 	}
// }
//
// func TestEventFromUnreadableRecords(t *testing.T) {
// 	tests := map[string]*kgo.Record{
// 		"no header":      {Value: []byte(`{"Ticker":"AAPL"}`)},
// 		"unknown kind":   {Value: []byte(`{"Ticker":"AAPL"}`), Headers: []kgo.RecordHeader{{Key: headerKind, Value: []byte("candle")}}},
// 		"a broken trade": {Value: []byte("not json"), Headers: []kgo.RecordHeader{{Key: headerKind, Value: []byte(kindTrade)}}},
// 		"a broken quote": {Value: []byte("not json"), Headers: []kgo.RecordHeader{{Key: headerKind, Value: []byte(kindQuote)}}},
// 	}
//
// 	for name, record := range tests {
// 		t.Run(name, func(t *testing.T) {
// 			if _, err := eventFromRecord(record); err == nil {
// 				t.Errorf("eventFromRecord(%s) error = nil, want an error", name)
// 			}
// 		})
// 	}
// }
//
// func TestTickFeedRunRefusesAnUnusableFeed(t *testing.T) {
// 	tests := map[string]TickFeed{
// 		"without a context": {Config: Config{Kafka: unreachableBroker, Tickers: []string{"aapl"}}},
// 		"without a ticker":  {Ctx: context.Background(), Config: Config{Kafka: unreachableBroker}},
// 	}
//
// 	for name, feed := range tests {
// 		t.Run(name, func(t *testing.T) {
// 			stream, err := feed.Run()
// 			if err == nil {
// 				t.Fatalf("Run() %s error = nil, want an error", name)
// 			}
//
// 			if stream != nil {
// 				t.Errorf("Run() %s returned a channel, want nil", name)
// 			}
// 		})
// 	}
// }
//
// // TestMassiveTickFeed is the round trip of the whole record shape: what
// // Publish writes for a ticker must come back out of the feed with the very
// // same fields, in an order whose timestamps never go backwards.
// func TestMassiveTickFeed(t *testing.T) {
// 	broker := requireBroker(t)
//
// 	ctx, cancel := context.WithTimeout(context.Background(), feedTimeout)
// 	defer cancel()
//
// 	ticker := throwawayTicker()
// 	producer := producerFor(t, broker)
//
// 	feed := &TickFeed{Ctx: ctx, Config: Config{Kafka: broker, Tickers: []string{string(ticker)}}}
// 	stream, err := feed.Run()
// 	if err != nil {
// 		t.Fatalf("Run() error = %v", err)
// 	}
//
// 	waitUntilConsuming(ctx, t, producer, ticker, stream)
//
// 	base := time.Now().UnixMilli()
// 	want := []gotrader.MarketEvent{
// 		quoteEvent(ticker, base, 1),
// 		tradeEvent(ticker, base, 2),
// 		quoteEvent(ticker, base+1, 3),
// 		tradeEvent(ticker, base+2, 4),
// 	}
//
// 	events := make(chan gotrader.MarketEvent, len(want))
// 	for _, event := range want {
// 		events <- event
// 	}
// 	close(events)
//
// 	if err = Publish(ctx, Config{Kafka: broker}, events); err != nil {
// 		t.Fatalf("Publish() error = %v", err)
// 	}
//
// 	var lastTS int64
// 	for i, expected := range want {
// 		got := nextEvent(ctx, t, stream)
//
// 		if diff := cmp.Diff(expected, got); diff != "" {
// 			t.Errorf("the event %d did not round trip (-want +got):\n%s", i, diff)
// 		}
//
// 		if got.TS() < lastTS {
// 			t.Errorf("timestamp went backwards: %d after %d", got.TS(), lastTS)
// 		}
// 		lastTS = got.TS()
// 	}
//
// 	cancel()
// 	requireClosedStream(t, stream)
// }
//
// // TestMassiveTickFeedStartsAtTheEnd is assumption A-3: the feed is live data,
// // not a replay. Whatever the topic already held when Run was called must never
// // reach the strategy.
// func TestMassiveTickFeedStartsAtTheEnd(t *testing.T) {
// 	broker := requireBroker(t)
//
// 	ctx, cancel := context.WithTimeout(context.Background(), feedTimeout)
// 	defer cancel()
//
// 	ticker := throwawayTicker()
// 	producer := producerFor(t, broker)
//
// 	history := time.Now().UnixMilli()
// 	for i := range 5 {
// 		produce(ctx, t, producer, tradeEvent(ticker, history+int64(i), int64(i)))
// 	}
//
// 	feed := &TickFeed{Ctx: ctx, Config: Config{Kafka: broker, Tickers: []string{string(ticker)}}}
// 	stream, err := feed.Run()
// 	if err != nil {
// 		t.Fatalf("Run() error = %v", err)
// 	}
//
// 	early := waitUntilConsuming(ctx, t, producer, ticker, stream)
// 	for _, event := range early {
// 		t.Errorf("the feed replayed an event published before Run(): %v", event.Trade)
// 	}
// }
//
// // TestMassiveTickFeedSkipsBadRecords puts in the topic what only a foreign
// // producer can write: a record with no kind header and one whose body is not a
// // market event. Neither may end the feed, and the valid record behind them
// // must still be delivered.
// func TestMassiveTickFeedSkipsBadRecords(t *testing.T) {
// 	broker := requireBroker(t)
//
// 	ctx, cancel := context.WithTimeout(context.Background(), feedTimeout)
// 	defer cancel()
//
// 	ticker := throwawayTicker()
// 	topic := topicFor(ticker)
// 	producer := producerFor(t, broker)
//
// 	produce(ctx, t, producer, tradeEvent(ticker, time.Now().UnixMilli(), 0))
//
// 	feed := &TickFeed{Ctx: ctx, Config: Config{Kafka: broker, Tickers: []string{string(ticker)}}}
// 	stream, err := feed.Run()
// 	if err != nil {
// 		t.Fatalf("Run() error = %v", err)
// 	}
//
// 	waitUntilConsuming(ctx, t, producer, ticker, stream)
//
// 	produceRecord(ctx, t, producer, &kgo.Record{Topic: topic, Key: []byte(topic), Value: []byte("not json"),
// 		Headers: []kgo.RecordHeader{{Key: headerKind, Value: []byte(kindTrade)}}})
// 	produceRecord(ctx, t, producer, &kgo.Record{Topic: topic, Key: []byte(topic), Value: []byte(`{"Ticker":"whatever"}`)})
//
// 	want := tradeEvent(ticker, time.Now().UnixMilli(), 42)
// 	produce(ctx, t, producer, want)
//
// 	if diff := cmp.Diff(want, nextEvent(ctx, t, stream)); diff != "" {
// 		t.Errorf("the record behind the corrupt ones was not delivered (-want +got):\n%s", diff)
// 	}
// }
//
// // requireBroker skips a test that needs a cluster when there is none.
// func requireBroker(t *testing.T) string {
// 	t.Helper()
//
// 	broker := os.Getenv("REDPANDA_BROKERS")
// 	if broker == "" {
// 		t.Skip("REDPANDA_BROKERS is not set, skipping the tick feed round trip")
// 	}
//
// 	return broker
// }
//
// // throwawayTicker names a topic no other run can collide with, so a test never
// // reads what another one wrote.
// func throwawayTicker() gotrader.Symbol {
// 	return gotrader.Symbol(fmt.Sprintf("test%d", time.Now().UnixNano()))
// }
//
// func tradeEvent(ticker gotrader.Symbol, ts, seq int64) gotrader.MarketEvent {
// 	return gotrader.MarketEvent{Trade: &gotrader.Trade{Ticker: ticker, TS: ts, Seq: seq, Price: 10, Size: 5, Exchange: 4, Tape: 1, Conditions: []int16{12}}}
// }
//
// func quoteEvent(ticker gotrader.Symbol, ts, seq int64) gotrader.MarketEvent {
// 	return gotrader.MarketEvent{Quote: &gotrader.Quote{Ticker: ticker, TS: ts, Seq: seq, BidPrice: 9, AskPrice: 10, BidSize: 1, AskSize: 2, BidExchange: 3, AskExchange: 4, Condition: 1, Indicators: []int16{7}, Tape: 2}}
// }
//
// // producerFor builds the raw client the tests write with. It is not Publish:
// // these tests also need to write records Publish would refuse to build.
// func producerFor(t *testing.T, broker string) *kgo.Client {
// 	t.Helper()
//
// 	client, err := kgo.NewClient(
// 		kgo.SeedBrokers(broker),
// 		kgo.AllowAutoTopicCreation(),
// 		kgo.RecordDeliveryTimeout(deliveryTimeout),
// 	)
// 	if err != nil {
// 		t.Fatalf("can not build the redpanda producer: %v", err)
// 	}
//
// 	t.Cleanup(client.Close)
// 	return client
// }
//
// func produce(ctx context.Context, t *testing.T, client *kgo.Client, event gotrader.MarketEvent) {
// 	t.Helper()
//
// 	record, err := recordFor(event)
// 	if err != nil {
// 		t.Fatalf("recordFor() error = %v", err)
// 	}
//
// 	produceRecord(ctx, t, client, record)
// }
//
// func produceRecord(ctx context.Context, t *testing.T, client *kgo.Client, record *kgo.Record) {
// 	t.Helper()
//
// 	if err := client.ProduceSync(ctx, record).FirstErr(); err != nil {
// 		t.Fatalf("can not write to the topic %s: %v", record.Topic, err)
// 	}
// }
//
// // waitUntilConsuming blocks until the feed proves it is live, by publishing
// // sentinel trades until one of them comes back on the stream. Without it the
// // sequence of a test races with the first fetch of a consumer that starts at
// // the end of the topic, and the assertions fail on a feed that works.
// //
// // It returns whatever the feed delivered before the first sentinel, which is
// // what the history test asserts is empty.
// func waitUntilConsuming(ctx context.Context, t *testing.T, client *kgo.Client, ticker gotrader.Symbol, stream <-chan gotrader.MarketEvent) []gotrader.MarketEvent {
// 	t.Helper()
//
// 	var early []gotrader.MarketEvent
//
// 	for {
// 		produce(ctx, t, client, tradeEvent(ticker, time.Now().UnixMilli(), sentinelSeq))
//
// 		select {
// 		case event, ok := <-stream:
// 			if !ok {
// 				t.Fatal("the feed closed its channel while waiting for it to go live")
// 			}
// 			if isSentinel(event) {
// 				return early
// 			}
// 			early = append(early, event)
//
// 		case <-time.After(warmupInterval):
//
// 		case <-ctx.Done():
// 			t.Fatal("the feed did not deliver any sentinel before the deadline")
// 		}
// 	}
// }
//
// // nextEvent reads the next event of the sequence under test, dropping the
// // sentinels still in flight when the feed went live.
// func nextEvent(ctx context.Context, t *testing.T, stream <-chan gotrader.MarketEvent) gotrader.MarketEvent {
// 	t.Helper()
//
// 	for {
// 		select {
// 		case event, ok := <-stream:
// 			if !ok {
// 				t.Fatal("the feed closed its channel before the sequence was delivered")
// 			}
// 			if isSentinel(event) {
// 				continue
// 			}
// 			return event
//
// 		case <-ctx.Done():
// 			t.Fatal("the feed did not deliver the whole sequence before the deadline")
// 		}
// 	}
// }
//
// func isSentinel(event gotrader.MarketEvent) bool {
// 	return event.IsTrade() && event.Trade.Seq == sentinelSeq
// }
//
// // requireClosedStream checks that a cancelled feed closes its channel instead
// // of leaking the goroutine that polls redpanda.
// func requireClosedStream(t *testing.T, stream <-chan gotrader.MarketEvent) {
// 	t.Helper()
//
// 	deadline := time.After(testTimeout)
// 	for {
// 		select {
// 		case _, ok := <-stream:
// 			if !ok {
// 				return
// 			}
//
// 		case <-deadline:
// 			t.Fatal("the feed did not close its channel after the context was cancelled")
// 		}
// 	}
// }
