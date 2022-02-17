package gotrader

import (
	"testing"
	"time"
)

func TestSignalGet(t *testing.T) {

	signals := MemorySignals{Metrics: map[string]*TimeSerie{}}

	c := Candle{
		Time:   time.Time{},
		Symbol: "SYMBL",
	}
	signals.Append(c, "test", 1.0)
	signals.Append(c, "test", 2.0)
	signals.Append(c, "test", 3.0)
	signals.Append(c, "test", 4.0)
	signals.Append(c, "test", 5.0)
	signals.Append(c, "test", 6.0)
	signals.Append(c, "test", 7.0)
	signals.Append(c, "test", 8.0)

	metrics := signals.GetMetrics()
	n, exists := metrics["SYMBL.test"]
	if !exists {
		t.Error("signal 'SYMBL.test' not found - note that the symbol is prependend to the metric!")
	}

	if len(n.X) != len(n.Y) {
		t.Errorf("len(n.X)[%v] != len(n.Y)[%v]", len(n.X), len(n.Y))
	}

	if len(n.X) != 8 {
		t.Error("len(n.X) != 7")
	}

	_, shouldErr := signals.Get(c, "blablah", 152)
	if shouldErr == nil {
		t.Errorf("signal blablah should not exts")
	}

	last, noErr := signals.Get(c, "test", 0)
	if noErr != nil || last != 8.0 {
		t.Errorf("expected last, got %f / an error: %v", last, noErr)
	}

	five, noErr := signals.Get(c, "test", 3)
	if noErr != nil || five != 5.0 {
		t.Errorf("expected last, got %f / an error: %v", five, noErr)
	}

	a, tooOld := signals.Get(c, "test", 357)
	if tooOld == nil {
		t.Errorf("expected a \"too old\" error, got a result: %f", a)
	}

}
