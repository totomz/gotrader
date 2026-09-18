# Task 08 - TradeStrategy and QuoteStrategy optional interfaces

Spec: `doc/datamodel.md` section 8.

## Scope
- In `strategy.go`: add `TradeStrategy` and `QuoteStrategy` interfaces with doc comments explaining that they are optional and detected by Cerbero via type assertion.
- Add `dispatchEvent(strategy Strategy, event MarketEvent)` (package-private helper) that calls `OnTrade` / `OnQuote` when implemented and does nothing otherwise.
- `Strategy` interface stays unchanged; existing strategies, examples and mocks must compile without edits.

## Out of scope
- Cerbero.Run changes.

## Definition of Done
`dispatchEvent` calls `OnTrade` only for strategies implementing `TradeStrategy` and `OnQuote` only for those implementing `QuoteStrategy`, verified by a unit test with three mocks (none, trade-only, both).

## Semantic tests
1. A mock implementing only `Strategy` receives no callback for a trade event and no callback for a quote event, and no panic occurs.
2. A mock implementing both interfaces receives exactly one `OnTrade` for a trade event and exactly one `OnQuote` for a quote event, with the same values carried by the event.
