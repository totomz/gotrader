# Task 09 - BacktestBrocker: fill on ticks with latency

Spec: `doc/datamodel.md` section 9.

## Scope
- In `broker.go`: add the `TickBroker` optional interface.
- `BacktestBrocker`: add `Latency time.Duration` (default 200ms when zero) and the private `lastEventTime`.
- `SubmitOrder`: set `SubmittedTime = lastEventTime` when `lastEventTime` is not zero.
- Extract the cash/portfolio/order-status update of `ProcessOrders` into a private `fill(order *Order, price float64, at time.Time, ctx)` used by both `ProcessOrders` (price = `candle.Open`) and `ProcessTrade` (price = `trade.Price`).
- `ProcessTrade(trade)`: update `lastEventTime`; skip trades with `UpdatesLast == false`; fill every pending order of the same symbol whose `SubmittedTime + Latency <= trade.Time()`.
- `ProcessQuote(quote)`: only update `lastEventTime`.
- Existing candle tests (`TestOrderExecutionAfter1sec`) must still pass unchanged.

## Out of scope
- Cerbero wiring, partial fills by size, slippage.

## Definition of Done
An order submitted while the broker clock is at `t0` is filled at the price of the first `UpdatesLast` trade of that symbol with `TS >= t0 + Latency`, and not before, verified by a unit test.

## Semantic tests
1. Broker with latency 200ms, clock at t0 via `ProcessQuote`. Submit a BUY of 1. Feed trades at t0+50ms (price 10), t0+150ms (price 11, `UpdatesLast=false`), t0+199ms (price 12), t0+200ms (price 13), t0+300ms (price 14). Expected: filled once at 13, cash decreased by 13, position size 1 with average price 13.
2. Two orders on two different symbols submitted at the same time: a trade of symbol A after latency fills only the order on A; the order on B stays accepted until a trade on B arrives.
