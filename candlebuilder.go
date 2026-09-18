package gotrader

import (
	"sort"
	"time"
)

const defaultSlotMs = int64(5000)

type candleState struct {
	slotStart int64
	open      float64
	high      float64
	low       float64
	close     float64
	volume    int64
	hasLast   bool
}

type CandleBuilder struct {
	SlotMs int64
	states map[Symbol]*candleState
}

func NewCandleBuilder(slotMs int64) *CandleBuilder {
	return &CandleBuilder{
		SlotMs: slotMs,
		states: map[Symbol]*candleState{},
	}
}

func (b *CandleBuilder) slotMs() int64 {
	if b.SlotMs <= 0 {
		return defaultSlotMs
	}
	return b.SlotMs
}

func (b *CandleBuilder) slotStartOf(ts int64) int64 {
	slot := b.slotMs()
	return ts / slot * slot
}

func (state *candleState) candle(symbol Symbol) Candle {
	return Candle{
		Open:   state.open,
		High:   state.high,
		Close:  state.close,
		Low:    state.low,
		Volume: state.volume,
		Symbol: symbol,
		Time:   time.UnixMilli(state.slotStart).UTC(),
	}
}

// Advance closes the candle of symbol when ts belongs to a following slot, and fills
// the slots with no trades with a carry-forward candle built on the last close.
func (b *CandleBuilder) Advance(symbol Symbol, ts int64) []Candle {
	state, found := b.states[symbol]
	if !found {
		return nil
	}

	slot := b.slotMs()
	if ts < state.slotStart+slot {
		return nil
	}

	target := b.slotStartOf(ts)
	closed := []Candle{state.candle(symbol)}
	lastClose := state.close

	for slotStart := state.slotStart + slot; slotStart < target; slotStart += slot {
		carried := candleState{slotStart: slotStart, open: lastClose, high: lastClose, low: lastClose, close: lastClose}
		closed = append(closed, carried.candle(symbol))
	}

	state.slotStart = target
	state.open = lastClose
	state.high = lastClose
	state.low = lastClose
	state.close = lastClose
	state.volume = 0
	state.hasLast = false

	return closed
}

func (b *CandleBuilder) Push(trade Trade) []Candle {
	closed := b.Advance(trade.Ticker, trade.TS)

	if !trade.UpdatesLast && !trade.UpdatesVolume {
		return closed
	}

	state, found := b.states[trade.Ticker]
	if !found {
		if !trade.UpdatesLast {
			return closed
		}

		state = &candleState{slotStart: b.slotStartOf(trade.TS)}
		if b.states == nil {
			b.states = map[Symbol]*candleState{}
		}
		b.states[trade.Ticker] = state
	}

	if trade.UpdatesLast {
		b.applyLast(state, trade.Price)
	}

	if trade.UpdatesVolume {
		state.volume += trade.Size
	}

	return closed
}

func (b *CandleBuilder) applyLast(state *candleState, price float64) {
	if !state.hasLast {
		state.open = price
		state.high = price
		state.low = price
		state.close = price
		state.hasLast = true
		return
	}

	if price > state.high {
		state.high = price
	}

	if price < state.low {
		state.low = price
	}

	state.close = price
}

func (b *CandleBuilder) Flush() []Candle {
	symbols := make([]Symbol, 0, len(b.states))
	for symbol := range b.states {
		symbols = append(symbols, symbol)
	}

	sort.Slice(symbols, func(i, j int) bool { return symbols[i] < symbols[j] })

	candles := make([]Candle, 0, len(symbols))
	for _, symbol := range symbols {
		candles = append(candles, b.states[symbol].candle(symbol))
	}

	b.states = map[Symbol]*candleState{}

	return candles
}
