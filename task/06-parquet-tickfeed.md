# Task 06 - massive.ParquetFeed (TickFeed implementation)

Spec: `doc/datamodel.md` sections 4, 5, 6.

## Scope
- In package `massive`: 

```go
type ParquetFeed struct {
    DataFolder string
    Day        time.Time
    Symbols    []gotrader.Symbol
}
func (f *ParquetFeed) Run() (chan gotrader.MarketEvent, error)
```

- For every symbol open the trades file (error if missing) and the quotes file (INFO log and skip if missing) using `PathFor`.
- Merge all readers into one stream with a k-way merge (heap keyed by `LessMarketEvent`), streaming, without loading whole files in memory.
- Close the output channel when every reader is exhausted.
- Channel buffer: 100_000.

## Out of scope
- Cerbero integration, realtime.

## Definition of Done
For a fixture day with two symbols (trades + quotes for both), `Run()` delivers every row exactly once, in `LessMarketEvent` order, then closes the channel; verified by a test.

## Semantic tests
1. Two symbols, interleaved timestamps, a trade and a quote of the same symbol sharing the same `ts`: the output contains all rows, the quote precedes the trade, and the sequence is globally non-decreasing in `ts`.
2. Symbol B has no quotes file: `Run()` succeeds, the stream contains A trades, A quotes, B trades, and an INFO line mentions the missing quotes for B.
