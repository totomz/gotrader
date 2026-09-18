# Task 03 - TickFeed interface

Spec: `doc/datamodel.md` section 5.

## Scope
- New file `tickfeed.go` in package `gotrader` with the `TickFeed` interface and its doc comment (ordering contract, channel closed at end of data, minimum buffer).
- Add an in-memory implementation `SliceTickFeed{Events []MarketEvent}` used by tests: it sorts the events with `LessMarketEvent` and pushes them.

## Out of scope
- Parquet, Cerbero.

## Definition of Done
`SliceTickFeed` implements `TickFeed`, emits the events sorted, and closes the channel; verified by a unit test that ranges over the channel to completion.

## Semantic tests
1. A `SliceTickFeed` built from unsorted events delivers them in `LessMarketEvent` order and the receiving loop terminates.
2. An empty `SliceTickFeed` returns a channel that is already closed (ranging over it yields nothing and does not block).
