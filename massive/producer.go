package massive

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"

	"github.com/totomz/gotrader"
	"github.com/twmb/franz-go/pkg/kgo"
)

const (
	// headerKind is the record header telling a trade from a quote, so a
	// consumer picks the right type without parsing the body first.
	headerKind = "kind"
	kindTrade  = "trade"
	kindQuote  = "quote"
)

// deliveryTimeout is how long a record may keep being retried before the
// client gives up on it. franz-go retries forever by default: against a broker
// that never comes back that is a producer wedged on its own buffer and a
// Publish that never returns. It is also what keeps the final flush bounded,
// since the records fail on their own instead of waiting for a deadline that
// would abandon them.
const deliveryTimeout = 30 * time.Second

// flushTimeout bounds the final flush. It is longer than deliveryTimeout on
// purpose: every record is failed by its own delivery timeout first, with its
// promise and its error, and the flush deadline is only the last resort.
const flushTimeout = deliveryTimeout + 10*time.Second

// WriteErrors reports the market events that did not reach redpanda. Publish
// returns it once the channel is drained and the client flushed, so a caller
// can tell a pipeline that wrote everything from one that dropped it all.
// The failures are also logged one by one as they happen.
type WriteErrors struct {
	// Failed is the number of events whose write failed.
	Failed int64

	// Sent is the number of events handed to the client, failures included.
	Sent int64
}

func (e WriteErrors) Error() string {
	return fmt.Sprintf("massive: %d market events of %d were not written to redpanda", e.Failed, e.Sent)
}

// Publish drains in and writes one record per event to the topic named after
// the ticker, in uppercase. The ticker is also the record key, so every event
// of one symbol lands on the same partition and keeps its relative order.
//
// It is deliberately ignorant of where the events came from: any channel does,
// which is what lets it be tested without a Massive connection. It returns
// when in is closed, which is how Ingest signals that it is done - including
// after a cancelled context, since Ingest closes the handoff channel on every
// exit path. Whoever writes on in owns closing it.
//
// A record that can not be built is logged and skipped. A write that fails is
// logged as it happens and the loop keeps draining: a broken broker must not
// turn into back-pressure on the ingest side. The last records are flushed
// before the client is closed, otherwise a shutdown loses them.
//
// It returns a WriteErrors when some event did not make it to the cluster.
// Unknown topics are created on first use when the broker allows it.
func Publish(ctx context.Context, cfg Config, in <-chan gotrader.MarketEvent) error {
	m, err := newMetrics()
	if err != nil {
		return err
	}

	client, err := kgo.NewClient(
		kgo.SeedBrokers(cfg.Kafka),
		kgo.AllowAutoTopicCreation(),
		kgo.RecordDeliveryTimeout(deliveryTimeout),
	)
	if err != nil {
		return fmt.Errorf("massive: can not build the redpanda client: %w", err)
	}
	defer client.Close()

	slog.Info("publishing the market events to redpanda", "broker", cfg.Kafka)

	return publish(ctx, client, in, m)
}

// publish is the drain loop of Publish over a client the caller owns, which is
// what lets a test produce against a client of its own - one with a short
// delivery timeout, for instance - without inventing a Config for it.
//
// The records are produced with a context that ignores the cancellation of
// ctx. Cancellation is how this pipeline is shut down, and franz-go fails
// every record buffered under a cancelled context: producing with ctx itself
// would throw away exactly the records the final flush is there to save. What
// bounds a record is the delivery timeout of the client, not the context.
func publish(ctx context.Context, client *kgo.Client, in <-chan gotrader.MarketEvent, m *metrics) error {
	produceCtx := context.WithoutCancel(ctx)

	var sent, failed atomic.Int64

	for event := range in {
		taken := time.Now()

		record, recordErr := recordFor(event)
		if recordErr != nil {
			slog.Error("skipping an event that can not be published", "ticker", event.Symbol(), "error", recordErr)
			continue
		}

		ticker, kind := event.Symbol(), kindOf(event)

		sent.Add(1)
		client.Produce(produceCtx, record, func(produced *kgo.Record, produceErr error) {
			if produceErr != nil {
				failed.Add(1)
				reason := writeErrorReason(produceErr)
				m.countWriteError(produceCtx, ticker, kind, reason)
				slog.Error("can not write the event to redpanda", "topic", produced.Topic, "reason", reason, "error", produceErr)
				return
			}

			m.countWritten(produceCtx, ticker, kind)
			m.recordWriteLatency(produceCtx, time.Since(taken), kind)
		})
	}

	flushCtx, cancel := context.WithTimeout(produceCtx, flushTimeout)
	defer cancel()

	if err := client.Flush(flushCtx); err != nil {
		return fmt.Errorf("massive: can not flush the pending records: %w", err)
	}

	writeErrors := WriteErrors{Failed: failed.Load(), Sent: sent.Load()}
	if writeErrors.Failed > 0 {
		return writeErrors
	}

	slog.Info("the redpanda producer stopped", "events", writeErrors.Sent)
	return nil
}

// kindOf tells a trade from a quote, so that the attribute of an instrument
// and the header of a record can never disagree. An event that is neither is
// reported as a trade here and refused by recordFor, which is the only place
// that has to reject it.
func kindOf(event gotrader.MarketEvent) string {
	if event.IsQuote() {
		return kindQuote
	}
	return kindTrade
}

// recordFor builds the redpanda record of one market event. The topic name is
// computed once and reused as the key: deriving the key on its own is how the
// two drift apart. The body is the json of the framework value, not of the
// pointer, so it reads back as an object and never as null.
func recordFor(event gotrader.MarketEvent) (*kgo.Record, error) {
	kind := kindTrade
	var payload any

	switch {
	case event.IsTrade():
		payload = *event.Trade
	case event.IsQuote():
		kind, payload = kindQuote, *event.Quote
	default:
		return nil, errors.New("massive: an event that is neither a trade nor a quote has nothing to publish")
	}

	topic := strings.ToUpper(string(event.Symbol()))
	if topic == "" {
		return nil, errors.New("massive: an event without a ticker has no topic")
	}

	value, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("massive: can not serialize the %s of %s: %w", kind, topic, err)
	}

	return &kgo.Record{
		Topic:   topic,
		Key:     []byte(topic),
		Value:   value,
		Headers: []kgo.RecordHeader{{Key: headerKind, Value: []byte(kind)}},
	}, nil
}
