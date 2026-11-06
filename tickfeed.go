package gotrader

const tickFeedBufferSize = 100_000

// TickFeed provides one ordered stream of trades and quotes for all the requested symbols.
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
