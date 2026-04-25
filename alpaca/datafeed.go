package alpacabroker

import (
	"context"
	"github.com/alpacahq/alpaca-trade-api-go/v3/marketdata"
	"github.com/alpacahq/alpaca-trade-api-go/v3/marketdata/stream"
	"github.com/totomz/gotrader"
	"log/slog"
)

// DataFeed is a live Alpaca datafeed that streams 1-minute bars via websocket.
type DataFeed struct {
	APIKey    string
	APISecret string
	Symbols   []gotrader.Symbol
	// Feed selects the data feed: marketdata.SIP (default), marketdata.IEX, marketdata.OTC
	Feed marketdata.Feed
}

func (d *DataFeed) Run() (chan gotrader.Candle, error) {
	feed := d.Feed
	if feed == "" {
		feed = marketdata.IEX
	}

	symbols := make([]string, len(d.Symbols))
	for i, s := range d.Symbols {
		symbols[i] = string(s)
	}

	out := make(chan gotrader.Candle, len(symbols)*10)

	handler := func(bar stream.Bar) {
		candle := gotrader.Candle{
			Symbol: gotrader.Symbol(bar.Symbol),
			Time:   bar.Timestamp,
			Open:   bar.Open,
			High:   bar.High,
			Low:    bar.Low,
			Close:  bar.Close,
			Volume: int64(bar.Volume),
		}

		select {
		case out <- candle:
		default:
			slog.Error("bar dropped", "reason", "channel full", "symbol", bar.Symbol)
		}
	}

	client := stream.NewStocksClient(
		feed,
		stream.WithCredentials(d.APIKey, d.APISecret),
		stream.WithBars(handler, symbols...),
	)

	if err := client.Connect(context.Background()); err != nil {
		return nil, err
	}

	go func() {
		if err := <-client.Terminated(); err != nil {
			slog.Error("alpaca stream terminated", "error", err)
		}
		close(out)
	}()

	slog.Info("alpaca datafeed started", "symbols", symbols, "feed", feed)
	return out, nil
}
