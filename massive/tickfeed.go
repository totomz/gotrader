package massive

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/totomz/gotrader"
	"github.com/twmb/franz-go/pkg/kgo"
	"log/slog"
)

const tickFeedBufferSize = 100_000

// TickFeed reads back the live stream that the massive ingester wrote
// to redpanda and presents it as a gotrader.TickFeed, so a strategy written
// for the parquet feeds runs on the live market unchanged.
type TickFeed struct {
	// Ctx stops the feed. Cancelling it ends the poll loop, closes the
	// redpanda client and closes the returned channel.
	Ctx context.Context

	Config Config
}

// Run subscribes to the topic of every configured ticker and starts a
// goroutine pushing the decoded market events on the returned channel. The
// channel is closed when Ctx is cancelled, which is the only way this feed
// ends
func (f *TickFeed) Run() (chan gotrader.MarketEvent, error) {
	if f.Ctx == nil {
		return nil, errors.New("massive: the tick feed has no context, it would never stop")
	}

	if len(f.Config.Tickers) == 0 {
		return nil, errors.New("massive: no ticker configured, there is no topic to consume")
	}

	client, err := kgo.NewClient(
		kgo.SeedBrokers(f.Config.Kafka),
		kgo.ConsumeTopics(f.Config.Tickers...),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtEnd()),
	)
	if err != nil {
		return nil, fmt.Errorf("massive: can not build the redpanda consumer: %w", err)
	}

	slog.Info("consuming the market events from redpanda", "broker", f.Config.Kafka, "topics", len(f.Config.Tickers))

	stream := make(chan gotrader.MarketEvent, tickFeedBufferSize)
	go f.consume(client, stream)

	return stream, nil
}

func (f *TickFeed) consume(client *kgo.Client, stream chan<- gotrader.MarketEvent) {
	defer close(stream)
	defer client.Close()

	var delivered int64

	for {
		fetches := client.PollFetches(f.Ctx)
		if fetches.IsClientClosed() || f.Ctx.Err() != nil {
			slog.Info("the redpanda tick feed stopped", "events", delivered)
			return
		}

		fetches.EachError(func(topic string, partition int32, err error) {
			slog.Error("can not fetch from redpanda", "topic", topic, "partition", partition, "error", err)
		})

		for _, record := range fetches.Records() {
			event, err := eventFromRecord(record)
			if err != nil {
				slog.Error("skipping an unreadable record", "topic", record.Topic, "offset", record.Offset, "error", err)
				continue
			}

			select {
			case <-f.Ctx.Done():
				slog.Info("the redpanda tick feed stopped", "events", delivered)
				return
			case stream <- event:
				delivered++
			}
		}
	}
}

// eventFromRecord turns one redpanda record back into the market event it was
// published from. The kind header decides the type to decode into, exactly as
// recordFor wrote it: the body alone can not tell a trade from a quote.
func eventFromRecord(record *kgo.Record) (gotrader.MarketEvent, error) {
	switch recordKind(record) {
	case kindTrade:
		var trade gotrader.Trade
		if err := json.Unmarshal(record.Value, &trade); err != nil {
			return gotrader.MarketEvent{}, fmt.Errorf("massive: can not decode a trade record: %w", err)
		}
		return gotrader.MarketEvent{Trade: &trade}, nil

	case kindQuote:
		var quote gotrader.Quote
		if err := json.Unmarshal(record.Value, &quote); err != nil {
			return gotrader.MarketEvent{}, fmt.Errorf("massive: can not decode a quote record: %w", err)
		}
		return gotrader.MarketEvent{Quote: &quote}, nil
	}

	return gotrader.MarketEvent{}, fmt.Errorf("massive: record without a usable %s header", headerKind)
}

// recordKind reads the kind header of a record, or an empty string when the
// header is not there.
func recordKind(record *kgo.Record) string {
	for _, header := range record.Headers {
		if header.Key == headerKind {
			return string(header.Value)
		}
	}
	return ""
}
