# Task 01 - Trade and Quote structs

Spec: `doc/datamodel.md` sections 1, 2, 3.

## Scope
- New file `tick.go` in package `gotrader`.
- Add `Trade` and `Quote` with exactly the fields and types listed in the datamodel.
- Add `Trade.Time()`, `Trade.String()`, `Quote.Time()`, `Quote.String()`, `Quote.Mid()`, `Quote.Spread()`, `Quote.SpreadRel()`, `Quote.IsValid()`.
- Reuse the existing `Symbol` type for `Ticker`.

## Out of scope
- Parsing, I/O, MarketEvent, feeds.

## Definition of Done
`Quote.IsValid()`, `Spread()`, `SpreadRel()` and `Mid()` behave exactly as the acceptance criteria in datamodel section 3, verified by a unit test in `tick_test.go`.

## Semantic tests (describe, do not code here)
1. A table of quotes covering: bid <= 0, ask <= 0, crossed (ask < bid), both sizes zero, locked (ask == bid), normal. Expected: only locked and normal are valid; locked has spread 0 and relative spread 0; normal has spread and relative spread computed from bid/ask.
2. A trade with `TS = 1700000000123` returns a `Time()` equal to that instant in UTC with millisecond precision and no rounding.
