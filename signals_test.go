package gotrader

import (
	"context"
	"go.opencensus.io/stats/view"
	"testing"
	"time"
)

type testExporter struct {
	hasMetric         bool
	expectMetricName  string
	expectMetricValue float64
}

func (ce *testExporter) hasMetrics() bool {
	return ce.hasMetric
}

func (ce *testExporter) ExportView(vd *view.Data) {
	if vd.View.Name != ce.expectMetricName {
		return
	}
	ce.hasMetric = true
	for _, row := range vd.Rows {
		d := row.Data.(*view.LastValueData)
		if d.Value == ce.expectMetricValue {
			ce.hasMetric = true
			break
		}
	}
}

func TestSimpleMetric(t *testing.T) {
	AMetric := NewMetricWithDefaultViews("test/ametric")

	z := &testExporter{
		expectMetricName:  AMetric.Name,
		expectMetricValue: 5.5,
	}

	view.RegisterExporter(z)
	view.SetReportingPeriod(100 * time.Millisecond)

	// Metrics without a candle are ignored by gotrader
	// but should work as opencensus measures/views
	ctxNoCandle := context.Background()
	AMetric.Record(ctxNoCandle, 5.5)

	time.Sleep(500 * time.Millisecond)

	if !z.hasMetrics() {
		t.Errorf("opencensus exporter didn't get the metric")
	}
}

func TestMetricCandles(t *testing.T) {
	AMetric := NewMetricWithDefaultViews("test/ametric2")
	t0 := time.Now()

	AMetric.Record(GetNewContextFromCandle(Candle{Symbol: "ZYO", Time: t0}), 1) // i := -3 (+3)
	t0 = t0.Add(1 * time.Second)
	AMetric.Record(GetNewContextFromCandle(Candle{Symbol: "ZYO", Time: t0}), 2) // i := -1 (+2)
	t0 = t0.Add(1 * time.Second)
	AMetric.Record(GetNewContextFromCandle(Candle{Symbol: "ZYO", Time: t0}), 3) // i := -1 (+1)
	t0 = t0.Add(1 * time.Second)
	AMetric.Record(GetNewContextFromCandle(Candle{Symbol: "ZYO", Time: t0}), 4) // i := 0

	// Metrics are bounded to a symbol; and there are no vlaues or this symbol
	_, err := AMetric.Get(GetNewContextFromCandle(Candle{Symbol: "XXX", Time: t0}), 1)
	if err == nil || err != ErrMetricNotFound {
		t.Errorf("expected an ErrMetricNotFound")
	}

	zyoCtx := GetNewContextFromCandle(Candle{Symbol: "ZYO", Time: t0})
	val4, err4 := AMetric.Get(zyoCtx, 0)
	if err4 != nil {
		t.Error(err4)
	}

	if val4 != 4 {
		t.Errorf("expected 4, got %f", val4)
	}

	val1, err1 := AMetric.Get(zyoCtx, 3)
	if err1 != nil {
		t.Error(err1)
	}

	if val1 != 1 {
		t.Errorf("expected 1, got %f", val4)
	}

	valN, errN := AMetric.Get(zyoCtx, -3)
	if errN != nil {
		t.Error(errN)
	}

	if valN != 1 {
		t.Errorf("expected 1, got %f", valN)
	}

	// This test is only to avoid a warning about unused metric
	// that is actually needed by strategies based on gotrader
	if MPosition.Name != "position" {
		t.Error("missing metric 'position")
	}
}
