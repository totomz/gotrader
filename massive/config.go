package massive

import (
	massivews "github.com/massive-com/client-go/v3/websocket"
)

type Config struct {
	APIKey string

	// Feed selects which stream to subscribe to, for example
	// massivews.RealTime
	Feed massivews.Feed

	// List of tickers to subscribe. An empty value is treated as `*`, "subscribe to everything"
	Tickers []string

	// Kafka is the host:port of the redpanda broker the events are written to
	Kafka string
}

// // Symbols returns the configured tickers as framework symbols, in the order
// // the caller wrote them.
// func (c Config) Symbols() []gotrader.Symbol {
// 	symbols := make([]gotrader.Symbol, len(c.Tickers))
// 	for i, ticker := range c.Tickers {
// 		symbols[i] = gotrader.Symbol(ticker)
// 	}
// 	return symbols
// }
