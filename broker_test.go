package gotrader

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func newTestBacktestBroker(cash float64, latency time.Duration) *BacktestBrocker {
	return &BacktestBrocker{
		BrokerAvailableCash: cash,
		OrderMap:            map[string]*Order{},
		Portfolio:           map[Symbol]Position{},
		EvalCommissions:     Nocommissions,
		Latency:             latency,
	}
}

func TestBacktestBrockerProcessTradeLatency(t *testing.T) {
	t.Parallel()

	t0 := time.Date(2023, 11, 14, 14, 30, 0, 0, time.UTC)
	broker := newTestBacktestBroker(1000, 200*time.Millisecond)

	broker.ProcessQuote(Quote{Ticker: testSymbol, TS: t0.UnixMilli(), BidPrice: 9.9, AskPrice: 10.1, BidSize: 100, AskSize: 100})

	orderID, err := broker.SubmitOrder(Candle{}, Order{Size: 1, Symbol: testSymbol, Type: OrderBuy})
	if err != nil {
		t.Fatalf("error submitting the order -- %v", err)
	}

	trades := []Trade{
		{Ticker: testSymbol, TS: t0.Add(50 * time.Millisecond).UnixMilli(), Price: 10, Size: 1, UpdatesLast: true},
		{Ticker: testSymbol, TS: t0.Add(150 * time.Millisecond).UnixMilli(), Price: 11, Size: 1, UpdatesLast: false},
		{Ticker: testSymbol, TS: t0.Add(199 * time.Millisecond).UnixMilli(), Price: 12, Size: 1, UpdatesLast: true},
		{Ticker: testSymbol, TS: t0.Add(200 * time.Millisecond).UnixMilli(), Price: 13, Size: 1, UpdatesLast: true},
		{Ticker: testSymbol, TS: t0.Add(300 * time.Millisecond).UnixMilli(), Price: 14, Size: 1, UpdatesLast: true},
	}

	var filled []Order
	for _, trade := range trades {
		filled = append(filled, broker.ProcessTrade(trade)...)
	}

	if len(filled) != 1 {
		t.Fatalf("expected 1 order filled, was %v", len(filled))
	}

	order, err := broker.GetOrderByID(orderID)
	if err != nil {
		t.Fatalf("error getting the order -- %v", err)
	}

	if order.Status != OrderStatusFullFilled {
		t.Errorf("expected the order to be OrderStatusFullFilled, was %v", order.Status)
	}

	if order.SizeFilled != 1 {
		t.Errorf("expected the order size filled to be 1, was %v", order.SizeFilled)
	}

	if order.AvgFilledPrice != 13 {
		t.Errorf("expected the order to be filled at 13, was %v", order.AvgFilledPrice)
	}

	if broker.AvailableCash() != 987 {
		t.Errorf("expected the available cash to be 987, was %v", broker.AvailableCash())
	}

	want := Position{Symbol: testSymbol, Size: 1, AvgPrice: 13}
	if diff := cmp.Diff(want, broker.GetPosition(testSymbol)); diff != "" {
		t.Errorf("position mismatch (-want +got):\n%s", diff)
	}
}

func TestBacktestBrockerProcessTradeSymbolIsolation(t *testing.T) {
	t.Parallel()

	const otherSymbol = Symbol("AAPL")
	t0 := time.Date(2023, 11, 14, 14, 30, 0, 0, time.UTC)
	broker := newTestBacktestBroker(1000, 200*time.Millisecond)

	broker.ProcessQuote(Quote{Ticker: testSymbol, TS: t0.UnixMilli(), BidPrice: 9.9, AskPrice: 10.1, BidSize: 100, AskSize: 100})

	orderA, err := broker.SubmitOrder(Candle{}, Order{Size: 1, Symbol: testSymbol, Type: OrderBuy})
	if err != nil {
		t.Fatalf("error submitting the order on %v -- %v", testSymbol, err)
	}

	orderB, err := broker.SubmitOrder(Candle{}, Order{Size: 1, Symbol: otherSymbol, Type: OrderBuy})
	if err != nil {
		t.Fatalf("error submitting the order on %v -- %v", otherSymbol, err)
	}

	broker.ProcessTrade(Trade{Ticker: testSymbol, TS: t0.Add(250 * time.Millisecond).UnixMilli(), Price: 20, Size: 1, UpdatesLast: true})

	filledA, err := broker.GetOrderByID(orderA)
	if err != nil {
		t.Fatalf("error getting the order on %v -- %v", testSymbol, err)
	}
	if filledA.Status != OrderStatusFullFilled {
		t.Errorf("expected the order on %v to be OrderStatusFullFilled, was %v", testSymbol, filledA.Status)
	}

	pendingB, err := broker.GetOrderByID(orderB)
	if err != nil {
		t.Fatalf("error getting the order on %v -- %v", otherSymbol, err)
	}
	if pendingB.Status != OrderStatusAccepted {
		t.Errorf("expected the order on %v to be OrderStatusAccepted, was %v", otherSymbol, pendingB.Status)
	}

	if diff := cmp.Diff(Position{}, broker.GetPosition(otherSymbol)); diff != "" {
		t.Errorf("expected no position on %v (-want +got):\n%s", otherSymbol, diff)
	}

	broker.ProcessTrade(Trade{Ticker: otherSymbol, TS: t0.Add(300 * time.Millisecond).UnixMilli(), Price: 30, Size: 1, UpdatesLast: true})

	filledB, err := broker.GetOrderByID(orderB)
	if err != nil {
		t.Fatalf("error getting the order on %v -- %v", otherSymbol, err)
	}
	if filledB.Status != OrderStatusFullFilled {
		t.Errorf("expected the order on %v to be OrderStatusFullFilled, was %v", otherSymbol, filledB.Status)
	}

	wantA := Position{Symbol: testSymbol, Size: 1, AvgPrice: 20}
	if diff := cmp.Diff(wantA, broker.GetPosition(testSymbol)); diff != "" {
		t.Errorf("position on %v mismatch (-want +got):\n%s", testSymbol, diff)
	}

	wantB := Position{Symbol: otherSymbol, Size: 1, AvgPrice: 30}
	if diff := cmp.Diff(wantB, broker.GetPosition(otherSymbol)); diff != "" {
		t.Errorf("position on %v mismatch (-want +got):\n%s", otherSymbol, diff)
	}
}
