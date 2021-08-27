package interactivebrokers

import (
	"errors"
	"fmt"
	"github.com/hadrianl/ibapi"
	"github.com/totomz/gotrader"
	"log"
	"strconv"
	"time"
)

type IbBroker struct {
	IBClient       *IbClientConnector
	contractsCache map[gotrader.Symbol]ibapi.Contract
}

func NewIbBrocker(ibClient *IbClientConnector) IbBroker {
	return IbBroker{
		IBClient:       ibClient,
		contractsCache: map[gotrader.Symbol]ibapi.Contract{},
	}
}

func (ib *IbBroker) SubmitOrder(order gotrader.Order) (string, error) {

	var orderType string
	switch t := order.Type; t {
	case gotrader.OrderBuy:
		orderType = "BUY"
	case gotrader.OrderSell:
		orderType = "SELL"
	default:
		panic("unknown order")
	}

	contract, err := ib.getIbContract(order.Symbol)
	if err != nil {
		return "", err
	}

	orderId, err := ib.IBClient.PlaceOrder(orderType, order.Size, contract)
	if err != nil {
		println(err)
		return "", err
	}

	return orderId, nil
}

func (ib *IbBroker) GetOrderByID(orderID string) (gotrader.Order, error) {
	dio, _ := strconv.ParseInt(orderID, 10, 64)

	// IbApi notifies the wrappe asynchronously...
	order, found := ib.IBClient.wrapper.orderCache[dio]
	if !found {
		// An orderId is generated after an order has been submitted, we can assume that
		// the order **should** exists. If it's not in my local cache, force a refresh and "wait for a little bit"
		ib.IBClient.api.ReqAllOpenOrders()
		time.Sleep(1 * time.Second)
		order, found = ib.IBClient.wrapper.orderCache[dio]
		if !found {
			return gotrader.Order{}, errors.New(fmt.Sprintf("order not found"))
		}
		return *order, nil
	}
	return *order, nil
}

func (ib *IbBroker) ProcessOrders(candle gotrader.Candle) {
	// Do nothing, the order is processed by the broker
}

func (ib *IbBroker) GetPosition(symbol gotrader.Symbol) (gotrader.Position, bool) {
	panic("NOT IMPLEMENTED")
}

func (ib *IbBroker) Shutdown() {
	panic("NOT IMPLEMENTED")
}

func (ib *IbBroker) AvailableCash() float64 {
	cash, err := ib.IBClient.AvailableFunds("All")
	if err != nil {
		log.Printf("error getting availableCash -- %v", err)
		return 0
	}

	return cash
}

func (ib *IbBroker) getIbContract(symbol gotrader.Symbol) (ibapi.Contract, error) {

	c, found := ib.contractsCache[symbol]
	if found {
		return c, nil
	}

	contracts, err := ib.IBClient.ReqContractDetails(ibapi.Contract{
		Symbol:       string(symbol),
		SecurityType: "STK",
		Currency:     "USD",
		Exchange:     "SMART",
	})

	if err != nil {
		return ibapi.Contract{}, err
	}
	if len(contracts) != 1 {
		return ibapi.Contract{}, fmt.Errorf("expecting 1 contract, foun %v", len(contracts))
	}

	c = contracts[0].Contract
	ib.contractsCache[symbol] = c
	return c, nil
}
