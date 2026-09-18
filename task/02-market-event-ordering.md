# Task 02 - MarketEvent and total ordering

Spec: `doc/datamodel.md` section 4.

## Scope
- In `tick.go` (package `gotrader`): add `MarketEvent` with helpers `TS()`, `Symbol()`, `IsTrade()`, `IsQuote()`.
- Add `LessMarketEvent(a, b MarketEvent) bool` implementing the four ordering rules.

## Out of scope
- Feeds, channels, Cerbero.

## Definition of Done
Sorting an arbitrary mixed slice of trade and quote events with `LessMarketEvent` produces the order defined in datamodel section 4, verified by a unit test.

## Semantic tests
1. Given events (in scrambled input order): trade TS=10 seq=2, quote TS=10 seq=9, trade TS=10 seq=1, quote TS=5 seq=50, trade AAPL TS=10 seq=1 vs trade MSFT TS=10 seq=1. Expected sorted order: quote TS=5, quote TS=10, trade TS=10 seq=1 AAPL, trade TS=10 seq=1 MSFT, trade TS=10 seq=2.
2. `LessMarketEvent(a, a)` is false for both a trade and a quote (strict weak ordering).
