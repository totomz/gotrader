package massive

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/massive-com/client-go/v3/websocket/models"
	"github.com/totomz/gotrader"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// wantInstruments is the instrument table of TS-04: the attribute keys of the
// counters carry the ticker, those of the histograms never do.
var wantInstruments = map[string][]string{
	"massive.events.received": {attrKind, attrTicker},
	"massive.events.written":  {attrKind, attrTicker},
	"massive.input.errors":    {attrKind, attrReason, attrTicker},
	"massive.write.errors":    {attrKind, attrReason, attrTicker},
	"massive.source.latency":  {attrKind},
	"massive.handoff.latency": {attrKind},
	"massive.write.latency":   {attrKind},
}

func TestInstruments(t *testing.T) {
	reader := installReader(t)

	m, err := newMetrics()
	if err != nil {
		t.Fatalf("newMetrics() error = %v", err)
	}

	ctx := context.Background()
	out := make(chan gotrader.MarketEvent, 3)
	marketTime := time.Now().Add(-time.Second).UnixMilli()

	frames := []any{
		models.EquityTrade{Symbol: "AAPL", Timestamp: marketTime, Price: 10, Size: 1},
		models.EquityQuote{Symbol: "MSFT", Timestamp: marketTime, BidPrice: 9, AskPrice: 10},
		"a frame this package does not know",
	}

	for _, frame := range frames {
		if handoffErr := handoff(ctx, m, frame, out); handoffErr != nil {
			t.Fatalf("handoff() error = %v", handoffErr)
		}
	}

	if len(out) != 2 {
		t.Errorf("the handoff channel holds %d events, want 2: the unknown frame must be skipped", len(out))
	}

	m.countWritten(ctx, "AAPL", kindTrade)
	m.countWritten(ctx, "MSFT", kindQuote)
	m.countWriteError(ctx, "AAPL", kindTrade, reasonTimeout)
	m.recordWriteLatency(ctx, time.Millisecond, kindTrade)

	collected := collectAttributes(ctx, t, reader)

	names := make([]string, 0, len(collected))
	for name := range collected {
		names = append(names, name)
	}
	slices.Sort(names)

	want := make([]string, 0, len(wantInstruments))
	for name := range wantInstruments {
		want = append(want, name)
	}
	slices.Sort(want)

	if diff := cmp.Diff(want, names); diff != "" {
		t.Errorf("the collected instruments are not the ones of the table (-want +got):\n%s", diff)
	}

	for name, sets := range collected {
		for _, set := range sets {
			if diff := cmp.Diff(wantInstruments[name], attributeKeys(set)); diff != "" {
				t.Errorf("the attributes of %s are wrong (-want +got):\n%s", name, diff)
			}
		}
	}

	if got := tickersOf(collected["massive.events.received"]); !slices.Equal(got, []string{"AAPL", "MSFT"}) {
		t.Errorf("the received counter reports the tickers %v, want [AAPL MSFT]", got)
	}

	if got := len(collected["massive.source.latency"]); got != 2 {
		t.Errorf("the source latency has %d series, want 2: one per kind and none per ticker", got)
	}
}

// installReader installs a manual reader as the global meter provider for the
// duration of one test, which is the only place in this repository where a
// provider is configured at all: the package itself never does it.
func installReader(t *testing.T) *sdkmetric.ManualReader {
	t.Helper()

	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(provider)
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })

	return reader
}

// collectAttributes reads the reader once and returns, per instrument name,
// the attribute set of every collected series. An instrument on which nothing
// was recorded is not collected at all and does not show up here.
func collectAttributes(ctx context.Context, t *testing.T, reader *sdkmetric.ManualReader) map[string][]attribute.Set {
	t.Helper()

	var collected metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &collected); err != nil {
		t.Fatalf("Collect() error = %v", err)
	}

	attributes := map[string][]attribute.Set{}
	for _, scope := range collected.ScopeMetrics {
		if scope.Scope.Name != meterName {
			t.Errorf("the instruments are on the scope %q, want %q", scope.Scope.Name, meterName)
		}

		for _, instrument := range scope.Metrics {
			switch data := instrument.Data.(type) {
			case metricdata.Sum[int64]:
				for _, point := range data.DataPoints {
					attributes[instrument.Name] = append(attributes[instrument.Name], point.Attributes)
				}

			case metricdata.Histogram[float64]:
				for _, point := range data.DataPoints {
					attributes[instrument.Name] = append(attributes[instrument.Name], point.Attributes)
				}

			default:
				t.Errorf("the instrument %s has the unexpected type %T", instrument.Name, data)
			}
		}
	}

	return attributes
}

// collectCounter returns the total of a counter over all its series.
func collectCounter(ctx context.Context, t *testing.T, reader *sdkmetric.ManualReader, name string) int64 {
	t.Helper()

	var collected metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &collected); err != nil {
		t.Fatalf("Collect() error = %v", err)
	}

	var total int64
	for _, scope := range collected.ScopeMetrics {
		for _, instrument := range scope.Metrics {
			sum, ok := instrument.Data.(metricdata.Sum[int64])
			if !ok || instrument.Name != name {
				continue
			}

			for _, point := range sum.DataPoints {
				total += point.Value
			}
		}
	}

	return total
}

// attributeKeys returns the sorted keys of an attribute set, which is what the
// cardinality rule of TS-04 is expressed on.
func attributeKeys(set attribute.Set) []string {
	keys := make([]string, 0, set.Len())
	for _, attr := range set.ToSlice() {
		keys = append(keys, string(attr.Key))
	}
	slices.Sort(keys)

	return keys
}

// tickersOf returns the sorted tickers of a group of series, so that a test
// can tell a per-ticker breakdown from a single aggregated figure.
func tickersOf(sets []attribute.Set) []string {
	tickers := make([]string, 0, len(sets))
	for _, set := range sets {
		if value, ok := set.Value(attribute.Key(attrTicker)); ok {
			tickers = append(tickers, value.AsString())
		}
	}
	slices.Sort(tickers)

	return tickers
}
