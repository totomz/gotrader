# Task 05 - Parquet quotes reader

Spec: `doc/datamodel.md` sections 3 and 6.

## Scope
- In `datafeed.go`: `readParquetQuotes(path string, out chan<- Quote) error`, same behaviour as `readParquetTrades` (streaming, monotonicity check, column mapping in one function).
- Test helper `writeQuotesFixture(t, path, quotes []Quote)`.

## Out of scope
- Merging, TickFeed implementation.

## Definition of Done
Reading a fixture file written by the helper returns the same quotes, in the same order, with every field equal (compared with `go-cmp`).

## Semantic tests
1. Round trip of quotes including an invalid one (crossed) and one with empty `Indicators`: all rows come back unchanged, the reader does not filter invalid quotes.
2. A fixture whose `ts` column is stored in nanoseconds by mistake (values > year 2200 in ms) is read as-is: the reader does not convert units, the check belongs upstream. Expected: values pass through unchanged.
