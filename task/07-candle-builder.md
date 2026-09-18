# Task 07 - CandleBuilder (5-second candles from trades)

Spec: `doc/datamodel.md` section 7.

## Scope
- New file `candlebuilder.go` in package `gotrader`:

```go
type CandleBuilder struct { SlotMs int64 /* default 5000 */ ... }
func NewCandleBuilder(slotMs int64) *CandleBuilder
// Advance closes and returns every candle of symbol whose slot ended before ts (carry-forward included).
func (b *CandleBuilder) Advance(symbol Symbol, ts int64) []Candle
// Push updates the open candle with the trade (after calling Advance internally).
func (b *CandleBuilder) Push(trade Trade) []Candle
// Flush closes the open candle of every symbol.
func (b *CandleBuilder) Flush() []Candle
```

- Pure logic, no I/O, no logging inside the hot path.

## Out of scope
- Cerbero wiring, quotes.

## Definition of Done
Given a fixed list of trades, the produced candles (values, `Time`, `Symbol`, count, carry-forward slots) equal the expected list, verified by a table-driven unit test.

## Semantic tests
1. Trades of one symbol at ms offsets 09:30:00.100, 09:30:03.900, 09:30:07.000, 09:30:21.000 (all `UpdatesLast` and `UpdatesVolume`). Expected on `Flush`: candle 09:30:00 (open first, close second, volume of the two), candle 09:30:05 (from the third), 09:30:10 and 09:30:15 carried forward with close of the previous and volume 0, candle 09:30:20 (from the fourth).
2. A trade with `UpdatesLast=false, UpdatesVolume=true` inside an open slot increases `Volume` but does not change `Open/High/Low/Close`; a trade with both flags false changes nothing.
