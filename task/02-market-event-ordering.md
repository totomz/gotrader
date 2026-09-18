# Task 02 - MarketEvent envelope

Spec: `doc/datamodel.md` section 4.

## Scope
- In `tick.go` (package `gotrader`): add `MarketEvent` with helpers `TS()`, `Symbol()`, `IsTrade()`, `IsQuote()`.
- No ordering or merging logic: that belongs to the feed implementations (task 06).

## Out of scope
- Feeds, channels, Cerbero.

## Definition of Done
`MarketEvent` helpers return the values of the wrapped trade or quote, verified by a unit test with one trade event and one quote event.

## Semantic tests
1. An event wrapping a trade: `IsTrade()` true, `IsQuote()` false, `TS()` and `Symbol()` equal to the trade's.
2. An event wrapping a quote: `IsQuote()` true, `IsTrade()` false, `TS()` and `Symbol()` equal to the quote's.
