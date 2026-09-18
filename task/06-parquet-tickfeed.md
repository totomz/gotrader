# Task 06 - GenericParquetTrades (TickFeed implementation)

Spec: `doc/datamodel.md` sections 4, 5 (ordering contract), 6.

## Scope
- In `datafeed.go` (package `gotrader`), vendor-neutral like `ZippedCSV`:

```go
type GenericParquetTrades struct {
    DataFolder string
    Day        time.Time
    Symbols    []Symbol
}
func (f *GenericParquetTrades) Run() (chan MarketEvent, error)
```

- For every symbol open the trades file (error if missing) and the quotes file (INFO log and skip if missing) using `parquetPathFor`.
- Private comparator `lessMarketEvent(a, b MarketEvent) bool` implementing the four ordering rules of section 5.
- Merge all readers into one stream with a k-way merge (heap keyed by `lessMarketEvent`), streaming, without loading whole files in memory.
- Close the output channel when every reader is exhausted.
- No filtering of any kind: every row of every file is delivered.
- Channel buffer: 100_000.

## Out of scope
- Cerbero integration, realtime.

## Definition of Done
For a fixture day with two symbols (trades + quotes for both), `Run()` delivers every row exactly once, in the order of section 5, then closes the channel; verified by a test.

## Semantic tests
0. Comparator: given scrambled events (trade TS=10 seq=2, quote TS=10 seq=9, trade TS=10 seq=1, quote TS=5, trade AAPL/MSFT TS=10 seq=1), sorting with `lessMarketEvent` yields: quote 5, quote 10, trade AAPL 10/1, trade MSFT 10/1, trade 10/2.
1. Two symbols, interleaved timestamps, a trade and a quote of the same symbol sharing the same `ts`: the output contains all rows, the quote precedes the trade, and the sequence is globally non-decreasing in `ts`.
2. Symbol B has no quotes file: `Run()` succeeds, the stream contains A trades, A quotes, B trades, and an INFO line mentions the missing quotes for B.
