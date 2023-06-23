package interactivebrokers

import (
	"github.com/totomz/gotrader"
	"testing"
)

func TestGetAvailableCash(t *testing.T) {
	ibClient, err := NewIbClientConnector(gateway, port, clientID)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ibClient.Close()
	})

	broker := IbBroker{
		IBClient: ibClient,
	}

	cash := broker.AvailableCash()
	if cash < 100000 {
		t.Error("WTF? Where is the money?")
	}

}

func TestGetOrderNX(t *testing.T) {
	ibClient, err := NewIbClientConnector(gateway, port, clientID)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ibClient.Close()
	})

	broker := NewIbBrocker(ibClient)

	_, err = broker.GetOrderByID("NANANANANA BATMAN")
	if err == nil || err.Error() != "order not found" {
		t.Errorf("expected an order not found, got [%v]", err)
	}
}

func TestSimpleOrder(t *testing.T) {

	// ORDERS ARE NOT CANCELLED because the broker api does not allow us to cancel an order :)
	// YOLO !!

	ibClient, err := NewIbClientConnector(gateway, port, clientID)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ibClient.Close()
	})

	broker := NewIbBrocker(ibClient)

	orderId, err := broker.SubmitOrder(gotrader.Order{
		Size:   1,
		Symbol: "NVOS",
		Type:   gotrader.OrderBuy,
	})

	if err != nil {
		t.Error(err)
	}

	order, err := broker.GetOrderByID(orderId)
	if err != nil {
		t.Errorf("expected an order - %v", err)
	}

	if order.Symbol != "NVOS" {
		t.Errorf("expected an order for 1 NVOS, got %v", order)
	}

}
