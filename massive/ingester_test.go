package massive

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	_ "github.com/joho/godotenv/autoload"
	massivews "github.com/massive-com/client-go/v3/websocket"
	"github.com/totomz/burrito/v2/common"
	"github.com/totomz/gotrader"
)

const testTimeout = 10 * time.Second

func TestIngester(t *testing.T) {
	apiKey, err := common.GetSecret(t.Context(), "bao://totomz/massive#key")
	if err != nil {
		t.Fatal(err)
	}
	if apiKey == "" {
		t.Skip("MASSIVE_API_KEY is not set, skipping the live ingest test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cfg := Config{
		APIKey:  apiKey,
		Feed:    massivews.RealTime,
		Tickers: []string{"AAPL"},
		Kafka:   "--",
	}
	events := make(chan gotrader.MarketEvent, 1024)
	done := make(chan error, 1)

	go func() {
		done <- Ingest(ctx, cfg, events)
	}()

	for received := 0; received < 1000; {
		select {
		case event := <-events:
			slog.Info("got event", "symbol", event.Symbol(), "is_quote", event.IsQuote(), "is_trade", event.IsTrade(), "event", event)
			if event.Trade != nil {
				// if event.Trade.Size != 0 {
				// 	println("ciocchino")
				// }
			}
			received++
			if event.Symbol() != "AAPL" {
				t.Errorf("got an event for %s, want AAPL", event.Symbol())
			}
		case <-ctx.Done():
			t.Fatalf("boh")
		}
	}

	cancel()

	select {
	case err := <-done:
		if err != nil && !errors.Is(err, context.Canceled) {
			slog.Error("the live ingest failed", "error", err)
		}
	case <-time.After(testTimeout):
		t.Fatal("Ingest() did not return after the context was cancelled")
	}

	assertClosed(t, events)
}

// assertClosed drains what is left in the handoff channel and fails the test
// when Ingest did not close it: a consumer ranging over it would hang.
func assertClosed(t *testing.T, events chan gotrader.MarketEvent) {
	t.Helper()

	for {
		select {
		case _, open := <-events:
			if !open {
				return
			}
		case <-time.After(testTimeout):
			t.Fatal("the handoff channel was not closed")
		}
	}
}

func TestIngestClosesTheHandoff(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		cfg    Config
		cancel time.Duration
	}{
		"no ticker configured": {
			cfg: Config{APIKey: "not-a-key", Feed: massivews.RealTime},
		},
		"the client can not be built": {
			cfg: Config{Feed: massivews.RealTime, Tickers: []string{"AAPL"}},
		},
		"the context is cancelled while connecting": {
			cfg:    Config{APIKey: "not-a-key", Feed: massivews.Feed("wss://127.0.0.1:1"), Tickers: []string{"AAPL"}},
			cancel: 200 * time.Millisecond,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			if test.cancel > 0 {
				time.AfterFunc(test.cancel, cancel)
			}

			events := make(chan gotrader.MarketEvent, 1)
			done := make(chan error, 1)

			go func() {
				done <- Ingest(ctx, test.cfg, events)
			}()

			select {
			case err := <-done:
				if err == nil {
					t.Error("Ingest() should fail")
				}
			case <-time.After(testTimeout):
				t.Fatal("Ingest() did not return")
			}

			assertClosed(t, events)
		})
	}
}
