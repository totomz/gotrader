# Tick data model (trades & quotes)

Reference spec for the tick-level support in gotrader. Every task in `task/` MUST be consistent with this file.
Decisions D1..D12 refer to the project decision table (5-second directional prediction, Massive data source).

## 1. Conventions

- All timestamps are **Unix milliseconds** (`int64`), D2. Truncation, never rounding.
- gotrader core (Cerbero, CandleBuilder, broker) **does not filter anything**: no RTH, tape or condition filtering. Filtering, when wanted, is done by the `TickFeed` implementation or by the strategy.
- The historical input of this project is the **normalized Parquet** (D10), one file per `date/ticker`, for trades and for quotes. It is read by a `TickFeed` implementation (`GenericParquetTrades`, in `datafeed.go`, vendor-neutral like `ZippedCSV`), not by the core.
- `UpdatesLast`, `UpdatesVolume`, `IsRTH` are **stored** in the Parquet, already derived upstream (D8). gotrader does not interpret SIP condition codes.

## 2. `Trade` (package `gotrader`, file `tick.go`)

| Field           | Type     | Parquet column   | Note                       |
|-----------------|----------|------------------|----------------------------|
| `Ticker`        | `Symbol` | `ticker`         |                            |
| `TS`            | `int64`  | `ts`             | SIP timestamp, ms          |
| `ParticipantTS` | `int64`  | `participant_ts` | ms                         |
| `Seq`           | `int64`  | `seq`            | tiebreak in ordering       |
| `Price`         | `float64`| `price`          |                            |
| `Size`          | `int64`  | `size`           |                            |
| `Exchange`      | `int16`  | `exchange`       | stored as INT32 in Parquet |
| `Tape`          | `int8`   | `tape`           | stored as INT32 in Parquet |
| `Conditions`    | `[]int16`| `conditions`     | LIST<INT32>, may be empty  |
| `UpdatesLast`   | `bool`   | `updates_last`   |                            |
| `UpdatesVolume` | `bool`   | `updates_volume` |                            |
| `IsRTH`         | `bool`   | `is_rth`         |                            |

Methods: `Time() time.Time` (returns `time.UnixMilli(TS).UTC()`), `String()`.

## 3. `Quote` (package `gotrader`, file `tick.go`)

| Field                        | Type      | Parquet column                 |
|------------------------------|-----------|--------------------------------|
| `Ticker`                     | `Symbol`  | `ticker`                       |
| `TS`                         | `int64`   | `ts`                           |
| `Seq`                        | `int64`   | `seq`                          |
| `BidPrice`, `AskPrice`       | `float64` | `bid_price`, `ask_price`       |
| `BidSize`, `AskSize`         | `int64`   | `bid_size`, `ask_size`         |
| `BidExchange`, `AskExchange` | `int16`   | `bid_exchange`, `ask_exchange` (INT32) |
| `Condition`                  | `int16`   | `condition` (INT32)            |
| `Indicators`                 | `[]int16` | `indicators` (LIST<INT32>)     |
| `Tape`                       | `int8`    | `tape` (INT32)                 |

Methods (not fields):

- `Mid() float64` = `(BidPrice + AskPrice) / 2`
- `Spread() float64` = `AskPrice - BidPrice`
- `SpreadRel() float64` = `Spread() / Mid()`; returns `0` when `Mid() == 0`
- `IsValid() bool` is **false** when `BidPrice <= 0`, or `AskPrice <= 0`, or `AskPrice < BidPrice` (crossed), or `BidSize == 0 && AskSize == 0`. A locked quote (`AskPrice == BidPrice`) **is valid** and has `Spread() == 0`.
- `Time() time.Time`, `String()`.

Invalid quotes are delivered to the strategy (so it can count them, feature `quote_invalid_count`) but they MUST NOT be used to update any NBBO book kept by gotrader.

## 4. `MarketEvent`

A Go channel carries one type, so the stream uses an envelope holding either a trade or a quote:

```go
// MarketEvent carries exactly one of Trade or Quote.
type MarketEvent struct {
    Trade *Trade
    Quote *Quote
}
```

Helpers: `TS() int64`, `Symbol() Symbol`, `IsTrade() bool`, `IsQuote() bool`. No ordering logic lives here.

## 5. `TickFeed` interface (file `tickfeed.go`)

```go
// TickFeed provides one ordered stream of trades and quotes for all the requested symbols.
type TickFeed interface {
    // Run starts a goroutine that pushes MarketEvent in the returned channel, ordered by time,
    // and closes the channel when the data is over. The buffer must hold at least 100_000 events.
    Run() (chan MarketEvent, error)
}
```

Ordering contract, responsibility of every implementation (Cerbero and strategies trust it):

1. `TS` ascending
2. same `TS`: `Quote` before `Trade` (the book state is known before the print)
3. same `TS` and same kind: `Seq` ascending
4. still equal: `Ticker` ascending (determinism across symbols)

Implementations: `GenericParquetTrades` in `datafeed.go` (historical, vendor-neutral). A Redpanda feed (realtime) will come later and is out of scope.

## 6. Parquet layout (`GenericParquetTrades`, file `datafeed.go`)

- Trades: `<DataFolder>/trades/<YYYY-MM-DD>/<TICKER>.parquet`
- Quotes: `<DataFolder>/quotes/<YYYY-MM-DD>/<TICKER>.parquet`
- The layout is produced by one function `parquetPathFor(kind, dataFolder, day, ticker) string` so it can be changed in one place.
- Columns as in sections 2 and 3, snake_case, physical types: `ticker` BYTE_ARRAY/UTF8, `ts`/`participant_ts`/`seq`/`size`/`bid_size`/`ask_size` INT64, prices DOUBLE, small ints INT32, lists LIST<INT32>, flags BOOLEAN.
- Rows inside a file are **already sorted** by `(ts, seq)`. The reader verifies monotonicity: an out-of-order row is skipped and logged with `slog.Error`, never re-sorted.
- The reader delivers every row as-is: no RTH/tape/condition filtering. `IsRTH` and the flags are passed through for the strategy to use.
- A missing quotes file for a ticker is **not** an error: the feed runs with trades only and logs it with `slog.Info`. A missing trades file **is** an error.

## 7. Candles from trades (file `candlebuilder.go`)

`CandleBuilder` turns trades into 5-second candles aligned to the grid, D5:

- `slotStart = floor(TS / 5000) * 5000`, `Candle.Time = time.UnixMilli(slotStart).UTC()`. The slot length is a parameter (`SlotMs int64`, default 5000).
- One builder state per `Symbol`.
- OHLC uses only trades with `UpdatesLast == true`. `Volume` sums `Size` of trades with `UpdatesVolume == true`. A trade with both flags false is ignored by the builder.
- A candle is **closed** when an event (trade or quote, same symbol) arrives with `TS >= slotStart + SlotMs`. Closing is triggered by `Push(trade)` / `Advance(symbol, ts)`.
- Empty slots between two candles of the same symbol are **carried forward**: `Open = High = Low = Close = previous Close`, `Volume = 0`. No candle is emitted before the first `UpdatesLast` trade of the symbol.
- `Flush()` closes the open candle of every symbol at end of stream.
- Pure, deterministic, no I/O (D9).

## 8. Strategy callbacks (file `strategy.go`)

`Strategy` is unchanged. Optional interfaces, detected with a type assertion by Cerbero:

```go
type TradeStrategy interface { OnTrade(trade Trade) }
type QuoteStrategy interface { OnQuote(quote Quote) }
```

A strategy may implement none, one or both. `Eval(candles)` keeps being called on every closed candle.

## 9. Backtest fill model with ticks (file `broker.go`)

Optional broker interface:

```go
type TickBroker interface {
    // ProcessTrade fills the pending orders that are eligible at this trade. Returns the orders touched.
    ProcessTrade(trade Trade) []Order
    // ProcessQuote only advances the broker clock.
    ProcessQuote(quote Quote)
}
```

`BacktestBrocker` implements it:

- `Latency time.Duration`, default `200ms` when zero (D12).
- The broker keeps `lastEventTime time.Time`, updated by `ProcessTrade`, `ProcessQuote` and `ProcessOrders`. `SubmitOrder` sets `order.SubmittedTime = lastEventTime` when it is not zero (the existing lazy assignment in `ProcessOrders` is kept as a fallback).
- An order is filled by the **first trade** of the same symbol with `UpdatesLast == true` and `TS >= SubmittedTime + Latency`, at `trade.Price`, for the full remaining size. Cash and portfolio are updated exactly like `ProcessOrders` does today (same code path, price parameter instead of `candle.Open`).
- Candle-based `ProcessOrders` behaviour is unchanged.

## 10. Cerbero tick pipeline (file `cerbero.go`)

New field `TickFeed TickFeed`. When it is not nil, `Run()` uses the tick pipeline and ignores `DataFeed` / `TimeAggregationFunc`. Setting both `DataFeed` and `TickFeed` returns an error.

For every `MarketEvent`, strictly in this order, in one goroutine:

1. Broker: `ProcessTrade` / `ProcessQuote` if the broker implements `TickBroker` (orders submitted at previous events may fill here).
2. Candle builder: `Advance(symbol, ts)`, then `Push(trade)` for trades. Every closed candle is appended to the candle history, tracked with `TrackCandleMetric`, and `Strategy.Eval(candles)` is called once per closed candle.
3. Strategy callback: `OnTrade` / `OnQuote` when implemented.

At end of stream: `Flush()` the builder (remaining candles go through step 2), then `Strategy.Shutdown()` and `Broker.Shutdown()`.
