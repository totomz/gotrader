package gotrader

import (
	"errors"
	"math"
	"sync"
	"testing"
	"time"
)

func TestBacktestBrocker_TestOrders(t *testing.T) {
	t.Parallel()

	broker := BacktestBrocker{
		InitialCashUSD:      30000,
		BrokerAvailableCash: 30000,
		OrderMap:            sync.Map{},
		Portfolio:           map[Symbol]Position{},
		EvalCommissions:     Nocommissions,
	}

	_, err := broker.GetOrderByID("order that does not exists")
	if !errors.Is(err, ErrOrderNotFound) {
		t.Fatal("Expected an error if the order is not found")
	}

	orderIdAmzn, err := broker.SubmitOrder(Order{
		Id:     "this will be changed",
		Size:   178,
		Symbol: "AMZN",
		Type:   OrderBuy,
	})
	if orderIdAmzn == "this will be changed" {
		t.Fatal("orderId has not been override for AMZN")
	}

	orderIdTsla, err := broker.SubmitOrder(Order{
		Id:     "this will be changed",
		Size:   456,
		Symbol: "TSLA",
		Type:   OrderSell,
	})
	if orderIdTsla == "this will be changed" {
		t.Fatal("orderId has not been override for TSLA")
	}

	amzn, err := broker.GetOrderByID(orderIdAmzn)
	if err != nil {
		t.Fatal(err)
	}

	if amzn.Symbol != "AMZN" {
		t.Fatalf("expected AMZN, got %s", amzn.Symbol)
	}

	if amzn.Status != OrderStatusAccepted {
		t.Fatalf("expected order in StautsAccepted, got %v", amzn.Status)
	}

	// Test a partially fulfilled order //
	broker.ProcessOrders(Candle{
		Open:   100,
		High:   110,
		Close:  101,
		Low:    90,
		Volume: 170,
		Symbol: "AMZN",
		Time:   time.Time{},
	})

	amznSemiFullfilled, err := broker.GetOrderByID(orderIdAmzn)
	if err != nil {
		t.Fatal(err)
	}

	if amznSemiFullfilled.Status != OrderStatusPartiallyFilled {
		t.Fatal("OrderStatus mismatch")
	}

	if amznSemiFullfilled.SizeFilled != 170 {
		t.Fatal("Partial execution mismatch")
	}

	broker.ProcessOrders(Candle{
		Open:   1024,
		High:   110,
		Close:  101,
		Low:    90,
		Volume: 200,
		Symbol: "AMZN",
		Time:   time.Time{},
	})

	amznDone, err := broker.GetOrderByID(orderIdAmzn)
	if err != nil {
		t.Fatal(err)
	}

	if amznDone.Status != OrderStatusFullFilled {
		t.Fatal("OrderStatus mismatch")
	}

	if amznDone.SizeFilled != 178 {
		t.Fatal("Order not fulfilled?")
	}

	position := broker.GetPosition("AMZN")
	if !almostEqual(position.AvgPrice, 141.52) {
		t.Fatalf("expected avg price was 141.52, got %v", position.AvgPrice)
	}
}

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) <= 1e-2
}
