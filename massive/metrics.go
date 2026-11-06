package massive

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/totomz/gotrader"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// meterName is the instrumentation scope of every instrument of this package.
const meterName = "github.com/totomz/gotrader/massive"

const (
	attrTicker = "ticker"
	attrKind   = "kind"
	attrReason = "reason"
)

// unknownAttr is the attribute value of a frame whose ticker or kind could not
// be read, which is the case of a frame the vendor SDK gave us and this
// package does not map.
const unknownAttr = "unknown"

// The reasons of an input error. They are a closed set on purpose: a raw error
// string here would make the cardinality of the counter unbounded.
const (
	reasonUnsupportedFrame = "unsupported_frame"
	reasonInvalidFrame     = "invalid_frame"
)

// The reasons of a write error, closed set for the same reason as the input
// ones. They map the failure modes franz-go reports to a produce promise.
const (
	reasonTimeout      = "timeout"
	reasonRetries      = "retries"
	reasonClientClosed = "client_closed"
	reasonCancelled    = "cancelled"
	reasonOther        = "other"
)

// metrics holds the instruments of the massive pipeline. The package creates
// them on the global meter and nothing else: installing a MeterProvider and an
// exporter is the job of the process that imports gotrader. With no provider
// installed the OTel default no-op takes over and the pipeline runs unchanged.
//
// Cardinality rule: ticker only on the counters, never on the histograms. A
// per-ticker histogram over a large universe is tens of thousands of series.
type metrics struct {
	received       metric.Int64Counter
	written        metric.Int64Counter
	inputErrors    metric.Int64Counter
	writeErrors    metric.Int64Counter
	sourceLatency  metric.Float64Histogram
	handoffLatency metric.Float64Histogram
	writeLatency   metric.Float64Histogram
}

// newMetrics builds the instrument set on the global meter. It is called once
// per Ingest and per Publish rather than kept in a package level variable, so
// that a test can install its own reader and collect only what it drove.
func newMetrics() (*metrics, error) {
	meter := otel.Meter(meterName)

	m := &metrics{}
	var err error

	if m.received, err = meter.Int64Counter("massive.events.received",
		metric.WithUnit("{event}"),
		metric.WithDescription("Trades and quotes received from the massive websocket")); err != nil {
		return nil, fmt.Errorf("massive: can not create the received counter: %w", err)
	}

	if m.written, err = meter.Int64Counter("massive.events.written",
		metric.WithUnit("{event}"),
		metric.WithDescription("Market events acknowledged by redpanda")); err != nil {
		return nil, fmt.Errorf("massive: can not create the written counter: %w", err)
	}

	if m.inputErrors, err = meter.Int64Counter("massive.input.errors",
		metric.WithUnit("{error}"),
		metric.WithDescription("Vendor frames that could not be mapped to a market event")); err != nil {
		return nil, fmt.Errorf("massive: can not create the input error counter: %w", err)
	}

	if m.writeErrors, err = meter.Int64Counter("massive.write.errors",
		metric.WithUnit("{error}"),
		metric.WithDescription("Market events that could not be written to redpanda")); err != nil {
		return nil, fmt.Errorf("massive: can not create the write error counter: %w", err)
	}

	if m.sourceLatency, err = meter.Float64Histogram("massive.source.latency",
		metric.WithUnit("s"),
		metric.WithDescription("Delay between the market timestamp of an event and its arrival in the process")); err != nil {
		return nil, fmt.Errorf("massive: can not create the source latency histogram: %w", err)
	}

	if m.handoffLatency, err = meter.Float64Histogram("massive.handoff.latency",
		metric.WithUnit("s"),
		metric.WithDescription("Delay between the arrival of an event in the process and its handoff to the publish half")); err != nil {
		return nil, fmt.Errorf("massive: can not create the handoff latency histogram: %w", err)
	}

	if m.writeLatency, err = meter.Float64Histogram("massive.write.latency",
		metric.WithUnit("s"),
		metric.WithDescription("Delay between reading an event from the handoff channel and its acknowledgement by redpanda")); err != nil {
		return nil, fmt.Errorf("massive: can not create the write latency histogram: %w", err)
	}

	return m, nil
}

// countReceived counts one event mapped out of the vendor feed.
func (m *metrics) countReceived(ctx context.Context, ticker gotrader.Symbol, kind string) {
	m.received.Add(ctx, 1, metric.WithAttributes(
		attribute.String(attrTicker, string(ticker)),
		attribute.String(attrKind, kind),
	))
}

// countWritten counts one event acknowledged by redpanda.
func (m *metrics) countWritten(ctx context.Context, ticker gotrader.Symbol, kind string) {
	m.written.Add(ctx, 1, metric.WithAttributes(
		attribute.String(attrTicker, string(ticker)),
		attribute.String(attrKind, kind),
	))
}

// countInputError counts one vendor frame this package could not use.
func (m *metrics) countInputError(ctx context.Context, ticker gotrader.Symbol, kind, reason string) {
	m.inputErrors.Add(ctx, 1, metric.WithAttributes(
		attribute.String(attrTicker, string(ticker)),
		attribute.String(attrKind, kind),
		attribute.String(attrReason, reason),
	))
}

// countWriteError counts one event that did not reach redpanda.
func (m *metrics) countWriteError(ctx context.Context, ticker gotrader.Symbol, kind, reason string) {
	m.writeErrors.Add(ctx, 1, metric.WithAttributes(
		attribute.String(attrTicker, string(ticker)),
		attribute.String(attrKind, kind),
		attribute.String(attrReason, reason),
	))
}

// recordSourceLatency records how long an event took from the timestamp the
// market put on it to its arrival in this process.
//
// The vendor timestamp is the exchange clock and the receive instant is the
// local one, so this delta also carries the skew between the two hosts and can
// come out negative: a negative value is dropped rather than clamped, because
// a zero would read as a feed with no latency at all.
func (m *metrics) recordSourceLatency(ctx context.Context, latency time.Duration, kind string) {
	if latency <= 0 {
		return
	}
	m.sourceLatency.Record(ctx, latency.Seconds(), metric.WithAttributes(attribute.String(attrKind, kind)))
}

// recordHandoffLatency records how long an event took from its arrival in the
// process to being handed to the publish half.
func (m *metrics) recordHandoffLatency(ctx context.Context, latency time.Duration, kind string) {
	m.handoffLatency.Record(ctx, latency.Seconds(), metric.WithAttributes(attribute.String(attrKind, kind)))
}

// recordWriteLatency records how long an event took from being read off the
// handoff channel to being acknowledged by redpanda.
func (m *metrics) recordWriteLatency(ctx context.Context, latency time.Duration, kind string) {
	m.writeLatency.Record(ctx, latency.Seconds(), metric.WithAttributes(attribute.String(attrKind, kind)))
}

// writeErrorReason classifies a produce failure into one of the fixed reasons
// of the write error counter.
func writeErrorReason(err error) string {
	switch {
	case errors.Is(err, kgo.ErrRecordTimeout):
		return reasonTimeout
	case errors.Is(err, kgo.ErrRecordRetries):
		return reasonRetries
	case errors.Is(err, kgo.ErrClientClosed), errors.Is(err, kgo.ErrAborting):
		return reasonClientClosed
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return reasonCancelled
	}
	return reasonOther
}
