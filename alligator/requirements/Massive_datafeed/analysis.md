# Massive Datafeed — Requirement Analysis

## Index

| § | Section |
|---|---|
| 1 | Requirement verbatim |
| 2 | Folding rules |
| 3 | Scope boundary |
| 4 | Actors / roles |
| 5 | Capabilities |
| 6 | Surface delta |
| 7 | Orchestration delta |
| 8 | Container / component impact |
| 9 | Data delta |
| 10 | External systems |
| 11 | Non-functional |
| 12 | Acceptance statements |
| 13 | Verification rows |
| 14 | Decisions needed |
| 15 | Assumptions |
| 16 | Open points |
| 17 | Coverage ledger |
| 18 | Grill table |

---

## 1. Requirement verbatim

| Ref | Text (verbatim, source `requirements/Massive_datafeed/epic-02-massive-datafeed.md`) |
|---|---|
| R-1 | "Aggiungi un nuovo datafeed per Massive.io realtime quotes and trades." |
| R-2 | "Il codice deve stare un in nuovo package `massive`. Il DataFeed deve essere configurato con una struct a cui passo i vari parametri idi configurazione" |
| R-3 | "Il datafeed deve connettersi al websocekt di massive e scaricare trades e quotes , gli passo iio la regexp di ticker da scaricare" |
| R-4 | "Poi deve scrivere i dati appena arrivati su una coda redpanda. Esiste un topic per ogni ticker, e si chiamano esattamente con il nome del ticker in uppercase, tipo AAPL" |
| R-5 | "Separa i due step con due func separate in modo che possano essere testate singolarmente, dire usando dei chan" |
| R-6 | "Scrivi un test per testare la func che si attacca al webhook." |
| R-7 | "scrivi un test per testare la func che scrive su redpanda anche senza assert, lo fa e non va in errore e va bene." |
| R-8 | "usa opentelemetry per esportare queste metriche" + 6 bullet: count per ticker · latenza di lettura · size del chan · errori input · errori scrittura · scritte in redpanda e latenza |

## 2. Folding rules

| Rule | Statement |
|---|---|
| F-1 | One capability = one named Go entry point the caller can invoke alone. Two funcs joined only by a channel are two capabilities. |
| F-2 | Instrumentation never folds into the capability it measures; all OTel instruments are one capability, because they share one meter and one naming convention. |
| F-3 | A type mapping folds into the capability that produces it, never its own row. |
| F-4 | Read path and write path are separate capabilities even over the same transport. |
| F-5 | A test is never a capability; it is a verification row against an acceptance statement. |
| F-6 | Config folds into the capability that consumes it unless two capabilities consume it — then it is its own data object. |

## 3. Scope boundary

| Object | Definition | Provenance | Status |
|---|---|---|---|
| `in-scope-ingest` | Package `massive` connects to Massive via SDK, maps to `gotrader.Trade`/`Quote`, hands off over a channel | R-1..R-3 | Confirmed |
| `in-scope-publish` | Same package produces JSON records to one Redpanda topic per ticker | R-4 | Confirmed |
| `in-scope-tickfeed` | `MassiveTickFeed` consumes those topics and implements the existing `TickFeed` interface, best-effort ordering | Grill R3-Q1 | Confirmed |
| `in-scope-otel` | OTel instruments created on the global meter; no provider, no exporter, no HTTP server in the library | R-8, Grill R6-Q1 | Confirmed |
| `out-of-scope-binary` | No `cmd/` entry point. Wiring, exporter and config loading belong to the caller's `main` | Grill R5-Q4 | Excluded — "gotrader è una libreria" |
| `out-of-scope-reorder` | No reordering buffer restoring the strict `tickfeed.go` ordering contract on the live feed | Grill R4-Q2 | Excluded — deferred to its own epic |
| `out-of-scope-vendor-fields` | Massive fields `i`, `ds`, `trfi`, `trft` are dropped, not modelled | Grill R5-Q1 | Excluded — "non ci servono" |
| `out-of-scope-parquet` | `tick.go` and the parquet readers/writers in `datafeed.go` are untouched | Consequence of the above | Excluded |
| `out-of-scope-replay` | No historical backfill, no replay from offset 0, no retention management | Not requested | Excluded |
| `out-of-scope-perf` | No load test, no throughput benchmark | Grill R6-Q3 | Excluded — `unbudgeted` |
| `out-of-scope-config-validation` | `Config` has no `Validate`: no required-field check, no duplicate-ticker check, no unit test. Wrong values surface at connection time | Grill R10-Q1 | Excluded — "leva pure la validazione" |
| `out-of-scope-regexp` | No ticker regexp, despite R-3. The SDK subscribes to an explicit list, so a pattern applied to a list the caller already typed adds a failure mode and no capability | Grill R7-Q1 | Excluded — supersedes R-3 |

## 4. Actors / roles

| Object | Definition | Provenance | Status |
|---|---|---|---|
| `quant-developer` | Writes the `main` that builds the config struct, starts ingest + publish and wires the OTel provider | R-2, Grill R6-Q1 | Confirmed |
| `strategy-runtime` | `cerbero.go` consuming a `TickFeed`; the caller of `MassiveTickFeed` | `cerbero.go`, `tickfeed.go` | Confirmed |
| `massive-feed` | External publisher of trade and quote frames | R-3 | Confirmed |

## 5. Capabilities

| Object | Definition | Provenance | Status |
|---|---|---|---|
| `configure-feed` | A `massive.Config` struct carrying API key, feed selector, the explicit ticker list and the Kafka broker address of the Redpanda cluster | R-2, Grill R7-Q1, R8-Q1, R9-Q1 | Confirmed |
| `ingest-market-stream` | Func that connects via `client-go/v3/websocket`, subscribes to the configured tickers, maps `models.EquityTrade`/`EquityQuote` to `gotrader.Trade`/`Quote` and pushes `MarketEvent` on an out channel | R-3, R-5, Grill R4-Q1 | Confirmed |
| `publish-to-redpanda` | Func that reads that channel and produces one JSON record per event to the ticker's topic, via franz-go | R-4, R-5, Grill R1-Q2 | Confirmed |
| `consume-tickfeed` | `MassiveTickFeed` implements `TickFeed.Run()` over the same topics, documented best-effort ordering | Grill R3-Q1, R4-Q2 | Confirmed |
| `observe-pipeline` | Counters, histograms and one observable gauge on `otel.Meter("gotrader/massive")` | R-8 | Confirmed |

## 6. Surface delta

| Object | Definition | Provenance | Status |
|---|---|---|---|
| `no-human-surface` | No screen, no route, no CLI. The only surface is the Go API of package `massive` | R-2, Grill R5-Q4 | Confirmed — N/A |

## 7. Orchestration delta

```
                          package massive
  ┌──────────────┐   ┌──────────────────────────────────────────┐   ┌──────────┐
  │ Massive WS   │──▶│ ingest-market-stream                     │   │ Redpanda │
  │ (SDK v3)     │   │  subscribe(cfg.Tickers)                  │   │          │
  └──────────────┘   │  map → gotrader.Trade / Quote            │   │ topic    │
     reconnect        │            │                             │   │  AAPL    │
     owned by SDK     │            ▼                             │   │  MSFT    │
                      │   chan MarketEvent (buffered 1e6)        │   │   ...    │
                      │            │                             │   └────▲─────┘
                      │            ▼                             │        │
                      │ publish-to-redpanda ─── JSON, key=ticker ┼────────┘
                      └──────────────────────────────────────────┘        │
                                                                          │
                      ┌──────────────────────────────────────────┐        │
  strategy-runtime ◀──│ consume-tickfeed → chan MarketEvent      │◀───────┘
                      └──────────────────────────────────────────┘
                           (best-effort ordering, not the strict
                            tickfeed.go contract)
```

| Object | Definition | Provenance | Status |
|---|---|---|---|
| `handoff-channel` | One plain `chan gotrader.MarketEvent` between the two funcs; it is the seam that makes each testable alone. No wrapper type, no pump goroutine | R-5, Grill R3-Q3, R11-Q1 | Confirmed |
| `handoff-buffer` | Buffered at `handoffBufferSize` = 1_000_000. It can not be unbounded: a Go channel allocates its buffer up front, so `math.MaxInt` panics with `makechan: size out of range`. When it fills, `Ingest` blocks on the send | Grill R11-Q1 | Confirmed — backpressure accepted, see OP-3 |

## 8. Container / component impact

| Object | Definition | Provenance | Status |
|---|---|---|---|
| `pkg-massive` | New package `massive/`: config, ingester, producer, tickfeed, metrics | R-2 | Add |
| `tickfeed-go` | `tickfeed.go` gains a doc note that `MassiveTickFeed` relaxes the ordering contract | Grill R4-Q2 | Modify — doc only |
| `go-mod` | Adds `github.com/massive-com/client-go/v3`, `github.com/twmb/franz-go`, `go.opentelemetry.io/otel` | Grill R1-Q2, R4-Q1 | Modify |
| `tick-go` | Untouched | Grill R5-Q1 | No change |
| `datafeed-go` | Untouched; `DataFeed`/`Candle` are not involved | Scope | No change |

## 9. Data delta

| Object | Definition | Provenance | Status |
|---|---|---|---|
| `massive-config` | Struct: `APIKey`, `Feed`, `Tickers []string`, `Kafka string` (one `host:port`), every field documented. `Market` is not a field: this feed is always stocks. No validation | R-2, Grill R7-Q1, R8-Q1, R10-Q1 | Add |
| `ticker-selection` | The subscription is exactly `Config.Tickers`. No regexp, no wildcard `*`, no symbol-list API call: the SDK's `Subscribe` takes an explicit list anyway, so a pattern would only be an intermediate step | Grill R1-Q3, superseded by R7-Q1 | Confirmed |
| `topic-name` | `strings.ToUpper(ticker)`, auto-created by the producer on first event | R-4, Grill R4-Q3 | Confirmed |
| `record-shape` | value = `encoding/json` of `gotrader.Trade` or `Quote`; key = ticker uppercase; header `kind` = `trade`\|`quote` | Grill R5-Q2 | Confirmed |

### 9.1 Trade field mapping (`models.EquityTrade` → `gotrader.Trade`)

| Massive | Type | gotrader | Status |
|---|---|---|---|
| `sym` | string | `Ticker` | Mapped |
| `t` | int64 ms | `TS` | Mapped |
| `pt` | int64 ms | `ParticipantTS` | Mapped |
| `q` | int64 | `Seq` | Mapped |
| `p` | float64 | `Price` | Mapped |
| `s` | int64 | `Size` | Mapped |
| `x` | int | `Exchange` (int16) | Mapped |
| `z` | int | `Tape` (int8) | Mapped |
| `c` | []int | `Conditions` ([]int16) | Mapped |
| `i`, `ds`, `trfi`, `trft` | — | — | Dropped (Grill R5-Q1) |
| — | — | `UpdatesLast`, `UpdatesVolume`, `IsRTH` | Zero value — see OP-1 |

### 9.2 Quote field mapping (`models.EquityQuote` → `gotrader.Quote`)

| Massive | gotrader | Status |
|---|---|---|
| `sym` / `t` / `q` / `z` | `Ticker` / `TS` / `Seq` / `Tape` | Mapped |
| `bp` / `bs` / `bx` | `BidPrice` / `BidSize` / `BidExchange` | Mapped |
| `ap` / `as` / `ax` | `AskPrice` / `AskSize` / `AskExchange` | Mapped |
| `c` / `i` | `Condition` / `Indicators` | Mapped |

### 9.3 OTel instruments

| Object | Instrument | Attributes | Provenance | Status |
|---|---|---|---|---|
| `events-received` | Counter | `ticker`, `kind` | R-8 b1, Grill R2-Q1 | Confirmed |
| `input-errors` | Counter | `ticker`, `kind`, `reason` | R-8 b4 | Confirmed |
| `events-written` | Counter | `ticker`, `kind` | R-8 b6 | Confirmed |
| `write-errors` | Counter | `ticker`, `kind`, `reason` | R-8 b5 | Confirmed |
| `handoff-latency` | Histogram | `kind` only | R-8 b2, Grill R2-Q2 | Confirmed — socket read → channel send, monotonic clock. With no queue-depth gauge, this is the saturation signal: it rises when the send blocks |
| `source-latency` | Histogram | `kind` only | Grill R2-Q2 | Confirmed — event `t` → local receive; clock-skew sensitive |
| `write-latency` | Histogram | `kind` only | R-8 b6 | Confirmed — channel receive → producer ack |

## 10. External systems

| Object | Definition | Provenance | Status |
|---|---|---|---|
| `massive-websocket` | `WS /stocks/T` and `WS /stocks/Q`, reached only through `client-go/v3/websocket`; auth, subscribe and reconnect owned by the SDK | Docs, Grill R1-Q1, R5-Q3 | Confirmed |
| `redpanda-cluster` | One Kafka-protocol broker addressed by `Config.Kafka` via franz-go. A single seed address, not a list: no multi-broker bootstrap redundancy | R-4, Grill R1-Q2, R8-Q1, R9-Q1 | Confirmed |
| `otel-meter` | `otel.Meter(...)` global; the library creates instruments only | Grill R6-Q1 | Confirmed |

## 11. Non-functional

| Object | Definition | Provenance | Status |
|---|---|---|---|
| `throughput` | No target | Grill R6-Q3 | `unbudgeted` |
| `latency-budget` | No target; measured, not bounded | Grill R6-Q3 | `unbudgeted` |
| `durability` | At-most-once: no producer retry policy specified beyond franz-go defaults | Not requested | `unbudgeted` |
| `cardinality` | `ticker` only on counters; histograms carry `kind` alone | Grill R2-Q1 | Confirmed |
| `availability` | Reconnect behaviour is whatever the SDK does | Grill R5-Q3 | Confirmed — not measured, see OP-2 |

## 12. Acceptance statements

| ID | Statement (resolvable object + observable outcome) |
|---|---|
| AS-1 | `massive.Config` exposes exactly four documented fields — API key, feed selector, ticker list and the `Kafka` broker address — each carrying a doc comment, and the ingester subscribes to exactly `Config.Tickers`. |
| AS-2 | The ingest func emits one `MarketEvent` on its out channel per Massive trade and quote frame, with every field of §9.1/§9.2 carried across. |
| AS-3 | The ingest func and the publish func are callable independently, joined only by a `chan MarketEvent`. |
| AS-4 | The publish func writes each event to the topic named `ToUpper(ticker)`, key = ticker, header `kind`, value = JSON, creating the topic if absent. |
| AS-5 | A malformed or unmappable input frame increments `input-errors` and is skipped; the ingester keeps running. |
| AS-6 | A producer failure increments `write-errors` and is logged with `slog.Error`; the publisher keeps running. |
| AS-7 | The seven instruments of §9.3 are registered on the global meter with the stated attributes, and no exporter or HTTP server is started by the library. |
| AS-8 | `MassiveTickFeed.Run()` returns a `chan MarketEvent` fed from the Redpanda topics, and its doc comment states that it guarantees only non-decreasing `TS` per ticker. |

## 13. Verification rows

| ID | Covers | Level | How |
|---|---|---|---|
| V-1 | AS-1 | --- | No test. The struct is proven by the fact that TS-02 and TS-03 compile against it and E2E-0201 connects with it (grill round 10). |
| V-2 | AS-2 | Unit | Table test on the two mapping funcs, `models.*` fixtures in, `Trade`/`Quote` out, `go-cmp` diff. |
| V-3 | AS-2, AS-3 | Integration | Real connection with `MASSIVE_API_KEY`; `t.Skip()` when unset; asserts ≥1 event lands on the channel (R-6, Grill R6-Q2). |
| V-4 | AS-3, AS-4 | Integration | Feed a hand-built channel to the publish func against a live broker from `REDPANDA_BROKERS`; `t.Skip()` when unset; passes if it returns no error (R-7). |
| V-5 | AS-5 | Unit | Inject an unmappable frame; assert the counter moved and the func did not return. |
| V-6 | AS-6 | Unit | Point the producer at an unreachable broker; assert the counter moved and the func did not return. |
| V-7 | AS-7 | Unit | Build the instrument set against a manual reader and assert the seven names and their attribute keys. |
| V-8 | AS-8 | Integration | Produce a known sequence, read it back through `MassiveTickFeed`, assert per-ticker `TS` is non-decreasing. |

## 14. Decisions needed

| ID | Decision | Status |
|---|---|---|
| — | None outstanding | Closed |

## 15. Assumptions

| ID | Assumption | Blocks if wrong |
|---|---|---|
| A-1 | `client-go/v3/websocket` exposes a quotes subscription constant and an `EquityQuote` model symmetrical to `StocksTrades`/`EquityTrade`; the README only documents the trade side. | `ingest-market-stream` |
| A-2 | The SDK's unified output channel delivers typed messages that a type switch can split into trades and quotes. | `ingest-market-stream` |
| A-3 | `MassiveTickFeed` consumes from the latest offset of each topic (live tail), not from the beginning. | `consume-tickfeed` |
| A-4 | Redpanda is configured to allow topic auto-creation; partition count and replication come from broker defaults. | `publish-to-redpanda` |
| A-5 | `handoff-latency` starts at the moment the SDK hands the message to our code, which is the earliest instant the library can observe. | `observe-pipeline` |

## 16. Open points

| ID | Open point | Blocks | Blocking decomposition |
|---|---|---|---|
| OP-1 | `UpdatesLast`, `UpdatesVolume`, `IsRTH` are not in the Massive WS payload; they could be derived from condition codes or left zero. | Fidelity of `gotrader.Trade` on the live path | No |
| OP-2 | The SDK's reconnect and resubscribe semantics are unverified, and no `reconnects` counter was requested. A silent permanent disconnect would be invisible. | `availability` | No |
| OP-3 | R-8 bullet 3 ("size del chan") has no instrument any more. When the 1e6 buffer fills, `Ingest` blocks on the send and stops reading the socket; the vendor may drop the session and those events are lost. The only signal is `handoff-latency` rising. | `handoff-buffer` | No |
| OP-4 | `requirements/Massive_datafeed/epic-02-massive-datafeed.md` is the raw requirement, not an epic; it will be superseded by the phase-2 epic file. | File layout | No |

## 17. Coverage ledger

| Dimension | State | Where |
|---|---|---|
| Scope boundary | Answered | §3 |
| Actor / role | Answered | §4 |
| Trigger | Answered | §5 `ingest-market-stream` — caller invokes the func |
| Happy path | Answered | §7 diagram |
| Alternate paths | Answered | §5 `consume-tickfeed` |
| Failure paths | Answered | AS-5, AS-6; OP-2, OP-3 |
| Preconditions / state | Answered | §9 `massive-config`, A-4 — unchecked by decision, see `out-of-scope-config-validation` |
| Data | Answered | §9.1, §9.2, §9.3 |
| Acceptance | Answered | §12 |
| Non-functional | Answered | §11 — three rows `unbudgeted` |
| Integrations | Answered | §10 |
| Migration / backfill | Excluded | §3 `out-of-scope-replay` |
| Rollout | Excluded | §3 `out-of-scope-binary` — caller owns deployment |
| Impact on existing behaviour | Answered | §8 — doc-only change to `tickfeed.go`, nothing else touched |
| Observability | Answered | §9.3 — seven instruments; R-8 bullet 3 dropped, see OP-3 |
| Test level | Answered | §13 |
| Harness / fixture | Answered | V-3 `MASSIVE_API_KEY` skip, V-4 `REDPANDA_BROKERS` skip |
| Regression surface | Answered | §8 — `tick.go`, `datafeed.go`, parquet path unchanged |

## 18. Grill table

| Round | Question | Answer verbatim | What it settled |
|---|---|---|---|
| 1 | How do I get the Massive WS spec? | "Mi dai tu la doc" | `massive-websocket`, §9.1, §9.2 |
| 1 | Which Kafka client? | "franz-go (consigliato)" | `publish-to-redpanda` |
| 1 | What universe does the regexp filter? | "lista di string da conf" | `ticker-selection`; later collapsed by R7-Q1 |
| 1 | Where do the files go? | "requirements/Massive_datafeed/" | File layout |
| 2 | Which metrics carry `ticker`? | "Solo sui contatori (consigliato)" | `cardinality`, §9.3 |
| 2 | What does read latency measure? | "Interna + exchange-to-ingest" | `handoff-latency`, `source-latency` |
| 2 | How to instrument channel size? | "ObservableGauge (consigliato)" | `queue-depth`; removed by R11-Q1 |
| 2 | How to export metrics? | "Prometheus /metrics" | Superseded by R6-Q1 |
| 3 | What does the package expose? | "Ingester + TickFeed" | `consume-tickfeed` |
| 3 | Payload and key on Redpanda? | "Raw passthrough" | Superseded by R5-Q2 |
| 3 | Behaviour when the channel is full? | "Chan illimitato" | Superseded by R11-Q1: one buffered chan, `handoff-buffer`, OP-3 |
| 3 | How does the Redpanda test run? | "Redpanda locale + skip (consigliato)" | V-4 |
| 4 | SDK deserializes, so raw passthrough is impossible — which one? | "SDK + Trade/Quote nostri E mappa i campi che mancano" | `ingest-market-stream`; later narrowed by R5-Q1 |
| 4 | Live feed cannot honour the `tickfeed.go` ordering contract | "Ordine best-effort documentato" | AS-8, `tickfeed-go` |
| 4 | Who creates the topics? | "Auto-create al primo evento (consigliato)" | `topic-name`, A-4 |
| 4 | Saturation, with no drop counter? | "Solo gauge depth" | Superseded by R11-Q1: no gauge at all, `handoff-latency` is the signal |
| 5 | Where do the extra Massive fields go? | "non mli mappare non ci servono" | `out-of-scope-vendor-fields`, `tick-go` No change |
| 5 | Payload now that types are ours? | "JSON, key=ticker, header kind (consigliato)" | `record-shape` |
| 5 | What happens when the socket drops? | "Delego all'SDK" | `availability`, OP-2 |
| 5 | Do we need a binary? | "Solo package" | `out-of-scope-binary` |
| 6 | Who serves /metrics? | "not your business to devi solo creare le metriche P gotrader è una libreria, ci pensa un main a fare il setup dell'exporter" | `otel-meter`, AS-7 |
| 6 | How to test the websocket func? | "Integrazione con API key vera" | V-3 |
| 6 | Throughput budget? | "Unbudgeted" | `throughput`, `latency-budget` |
| 7 | Why do `Tickers` and `TickerRegexp` coexist in Config? | "Solo Tickers []string" | `massive-config`, `ticker-selection`, AS-1, V-1 — the regexp of R-3 is dropped: the SDK subscribes to an explicit list, so a pattern over a list the caller already wrote is a step with no purpose |
| 8 | What are Feed, Market and Brokers? | "Market: è sempre stocks" · "Brokers: chiamiamolo kafka" | `massive-config` loses `Market`, which is hardcoded to `massivews.Stocks`; `Brokers` is renamed `Kafka` |
| 9 | Why is `Kafka` a slice? | "eh porcoddio si basta una string" | `massive-config`, `redpanda-cluster` — `Kafka` becomes one `host:port` string; franz-go gets a single seed broker and loses multi-seed bootstrap redundancy |
| 10 | (unprompted) | "leva i test dalla Config, non serve a un cazzo" · "leva pure la validazione" · "metti i commenti ai field della struct doc" | `out-of-scope-config-validation`; TS-01 loses `Validate` and UT-0101, keeps the struct with a doc comment per field; AS-1 and V-1 rewritten |
| 11 | Why a pump goroutine instead of one chan? | "ma chi cazzo ha detto che è buffered [...] è un chan unbuffered, nessuno aspetta un cazzo" · "leva la metrica e metti la size a MaxInt" | `handoff-channel` is one plain chan, the `unboundedQueue` type is gone; `queue-depth` is removed, leaving seven instruments. `math.MaxInt` panics at `make`, so `handoff-buffer` is 1_000_000 |
