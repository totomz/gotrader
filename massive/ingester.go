package massive

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	massivews "github.com/massive-com/client-go/v3/websocket"
	"github.com/massive-com/client-go/v3/websocket/models"
	"github.com/totomz/gotrader"
)

// HandoffBufferSize is the buffer of the channel between Ingest and Publish.
const HandoffBufferSize = 1_000_000

// shutdownTimeout bounds how long Ingest waits for the vendor client to close
// before it walks away from it.
const shutdownTimeout = 5 * time.Second

// Ingest connects to Massive, subscribes to the configured tickers and sends
// every trade and quote on out as a framework MarketEvent.
func Ingest(ctx context.Context, cfg Config, out chan<- gotrader.MarketEvent) error {
	defer close(out)

	if len(cfg.Tickers) == 0 {
		return fmt.Errorf("massive: no ticker configured, refusing to subscribe to the whole market")
	}

	m, err := newMetrics()
	if err != nil {
		return err
	}

	client, err := massivews.New(massivews.Config{
		APIKey: cfg.APIKey,
		Feed:   cfg.Feed,
		Market: massivews.Stocks,
	})
	if err != nil {
		return fmt.Errorf("massive: can not build the websocket client: %w", err)
	}

	connected := make(chan error, 1)
	go func() {
		connected <- client.Connect()
	}()

	select {
	case <-ctx.Done():
		go shutdownWhenIdle(client, connected)
		return ctx.Err()

	case err = <-connected:
		if err != nil {
			shutdown(client)
			return fmt.Errorf("massive: can not connect to the websocket: %w", err)
		}
	}

	defer shutdown(client)

	if err = client.Subscribe(massivews.StocksTrades, cfg.Tickers...); err != nil {
		return fmt.Errorf("massive: can not subscribe to the trades: %w", err)
	}

	if err = client.Subscribe(massivews.StocksQuotes, cfg.Tickers...); err != nil {
		return fmt.Errorf("massive: can not subscribe to the quotes: %w", err)
	}

	slog.Info("subscribed to the massive feed", "tickers", len(cfg.Tickers))

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case vendorErr := <-client.Error():
			return fmt.Errorf("massive: the websocket client failed: %w", vendorErr)

		case msg, ok := <-client.Output():
			if !ok {
				return fmt.Errorf("massive: the websocket client closed its output channel")
			}

			if handoffErr := handoff(ctx, m, msg, out); handoffErr != nil {
				return handoffErr
			}
		}
	}
}

// handoff maps one vendor frame, measures it and sends it to the publish
// half. A frame that can not be mapped is counted, logged and skipped: it is
// an input error, not a reason to stop ingesting. The only error it returns is
// the cancellation of ctx while the handoff channel is full.
//
// The receive instant is taken once and used by both latencies: the source one
// ends where the handoff one starts, so the two never overlap.
func handoff(ctx context.Context, m *metrics, msg any, out chan<- gotrader.MarketEvent) error {
	received := time.Now()

	event, mapErr := eventFromMessage(msg)
	if mapErr != nil {
		ticker, kind := frameAttributes(msg)
		reason := reasonInvalidFrame
		if kind == unknownAttr {
			reason = reasonUnsupportedFrame
		}

		m.countInputError(ctx, ticker, kind, reason)
		slog.Error("skipping an unusable frame", "reason", reason, "error", mapErr)
		return nil
	}

	kind := kindOf(event)
	m.recordSourceLatency(ctx, received.Sub(time.UnixMilli(event.TS())), kind)

	select {
	case <-ctx.Done():
		return ctx.Err()
	case out <- event:
	}

	m.countReceived(ctx, event.Symbol(), kind)
	m.recordHandoffLatency(ctx, time.Since(received), kind)

	return nil
}

// frameAttributes reads the ticker and the kind of a frame that could not be
// mapped, so that an input error is still attributed to the symbol that caused
// it. A frame this package does not know, or one whose symbol is missing, is
// reported as unknown rather than as an empty attribute.
func frameAttributes(msg any) (gotrader.Symbol, string) {
	switch frame := msg.(type) {
	case models.EquityTrade:
		return tickerAttribute(frame.Symbol), kindTrade
	case models.EquityQuote:
		return tickerAttribute(frame.Symbol), kindQuote
	}

	return unknownAttr, unknownAttr
}

// tickerAttribute keeps the ticker attribute out of the empty string, which
// reads as a bug in a dashboard rather than as a frame without a symbol.
func tickerAttribute(symbol string) gotrader.Symbol {
	if symbol == "" {
		return unknownAttr
	}
	return gotrader.Symbol(symbol)
}

func shutdownWhenIdle(client *massivews.Client, connected <-chan error) {
	<-connected
	shutdown(client)
}

func shutdown(client *massivews.Client) {
	closed := make(chan struct{})
	go func() {
		defer close(closed)
		client.Close()
	}()

	deadline := time.NewTimer(shutdownTimeout)
	defer deadline.Stop()

	for {
		select {
		case <-closed:
			return

		case <-client.Error():

		case <-deadline.C:
			slog.Error("the massive client did not close in time, abandoning it", "timeout", shutdownTimeout)
			return
		}
	}
}
