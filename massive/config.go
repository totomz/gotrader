package massive

import (
	massivews "github.com/massive-com/client-go/v3/websocket"
	"github.com/totomz/gotrader"
)

// Config holds every parameter of a Massive realtime feed.
//
// The caller owns where these values come from: gotrader is a library and
// never reads the environment on its own. The struct is passed by value to
// Ingest, Publish and MassiveTickFeed, each of which reads the fields it
// needs. Nothing here is validated: a wrong value surfaces as a connection
// error where it is used.
type Config struct {
	// APIKey authenticates the websocket session with Massive. The SDK sends
	// it during its own handshake; this package never logs it.
	APIKey string

	// Feed selects which stream to subscribe to, for example
	// massivews.RealTime. It is the SDK's own type on purpose, so the caller
	// does not have to learn a second vocabulary for the same concept.
	// The market is not configurable: this feed is always stocks.
	Feed massivews.Feed

	// Tickers is the exact subscription: these symbols and no others. There
	// is no pattern and no wildcard, because the SDK subscribes to an
	// explicit list anyway. The strings reach the vendor as written here;
	// the uppercasing happens only when the redpanda topic name is built.
	Tickers []string

	// Kafka is the host:port of the redpanda broker the events are written
	// to, and the one MassiveTickFeed reads them back from. One seed
	// address, not a list: there is no multi-broker bootstrap.
	Kafka string
}

// Symbols returns the configured tickers as framework symbols, in the order
// the caller wrote them.
func (c Config) Symbols() []gotrader.Symbol {
	symbols := make([]gotrader.Symbol, len(c.Tickers))
	for i, ticker := range c.Tickers {
		symbols[i] = gotrader.Symbol(ticker)
	}
	return symbols
}
