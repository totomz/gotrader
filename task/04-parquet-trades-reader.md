# Task 04 - Parquet trades reader

Spec: `doc/datamodel.md` sections 2 and 6.

## Scope
- Code goes in `datafeed.go` (package `gotrader`), next to `ZippedCSV`. No vendor package.
- Add the Parquet dependency `github.com/parquet-go/parquet-go` to `go.mod`. Verify the API against the module source in the module cache, do not assume it.
- `parquetPathFor(kind, dataFolder string, day time.Time, ticker Symbol) string` (kind is `"trades"` or `"quotes"`), the only place where the layout is defined.
- `readParquetTrades(path string, out chan<- Trade) error`: streams the rows of one trades file into `out` (row-group by row-group, not the whole file in memory), maps columns to `Trade`, verifies `(ts, seq)` monotonicity (skip + `slog.Error` on violation). No RTH/tape/condition filtering. Closes nothing: the caller owns `out`.
- Test helper `writeTradesFixture(t, path, trades []Trade)` that writes a Parquet file with the schema of section 6, used by tests of this and later tasks.

## Out of scope
- Quotes, merging, TickFeed implementation.

## Definition of Done
Reading a fixture file written by the helper returns the same trades, in the same order, with every field equal (compared with `go-cmp`).

## Semantic tests
1. Round trip: 5 trades with mixed `Conditions` (empty and non-empty), flags true/false, are written and read back identical.
2. A fixture where the 3rd row has a smaller `ts` than the 2nd: the reader returns 4 trades, the out-of-order row is missing, and an ERROR log line was emitted.
