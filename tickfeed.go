package gotrader

const tickFeedBufferSize = 100_000

// TickFeed provides one ordered stream of trades and quotes for all the requested symbols.
//
// Run starts a goroutine that pushes MarketEvent in the returned channel, ordered by time,
// and closes the channel when the data is over. The buffer must hold at least 100_000 events.
//
// The ordering contract is the responsibility of every implementation:
//  1. TS ascending
//  2. same TS: Quote before Trade (the book state is known before the print)
//  3. same TS and same kind: Seq ascending
//  4. still equal: Ticker ascending (determinism across symbols)
type TickFeed interface {
	Run() (chan MarketEvent, error)
}

type SliceTickFeed struct {
	Events []MarketEvent
}

func (f *SliceTickFeed) Run() (chan MarketEvent, error) {
	stream := make(chan MarketEvent, tickFeedBufferSize)

	go func() {
		defer close(stream)
		for _, event := range f.Events {
			stream <- event
		}
	}()

	return stream, nil
}
