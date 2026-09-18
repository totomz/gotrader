# Task 10 - Cerbero tick pipeline

Spec: `doc/datamodel.md` section 10.

## Scope
- In `cerbero.go`: add field `TickFeed TickFeed`.
- `Run()`: if both `DataFeed` and `TickFeed` are set return an error; if `TickFeed` is set execute the tick pipeline (private `runTicks`) with the exact per-event order of section 10 (broker, candle builder + `Eval`, strategy callback), single goroutine consuming the channel.
- Use `CandleBuilder` with the default 5000 ms slot (exposed as `Cerbero.CandleSlotMs`, default when zero).
- At end of stream: flush the builder, then `Strategy.Shutdown()` and `Broker.Shutdown()`. `ExecutionResult` is computed as today.
- Candle pipeline behaviour must remain byte-for-byte the same (existing tests pass).

## Out of scope
- Realtime feeds, metrics beyond `TrackCandleMetric`.

## Definition of Done
An end-to-end run with `SliceTickFeed`, `BacktestBrocker` and a mock strategy produces the expected sequence of callbacks (`Eval`, `OnTrade`, `OnQuote`) and order fills, verified by an integration test in `cerbero_test.go`.

## Semantic tests
1. Feed: quote at 09:30:00.000, trades at 09:30:01, 09:30:03, 09:30:06 (one symbol). Expected callback sequence recorded by the mock: OnQuote, OnTrade, OnTrade, Eval(1 candle for slot 09:30:00), OnTrade(09:30:06). After the stream: Eval(2 candles) from the flush, then Shutdown.
2. The mock submits a BUY inside `OnTrade` at 09:30:01. With latency 200ms, the order is filled by the trade at 09:30:03 and the fill is visible (position size 1) when `OnTrade(09:30:03)` is invoked, since the broker step runs before the strategy callback.
