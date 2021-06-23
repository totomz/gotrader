package gotrader

import "log"

type Symbol string

type OrderType int

const (
	OrderBuy = iota
	OrderSell
	OrderClose
)

type OrderStatus int

const (
	Submitted = iota
	Accepted
	FullFilled
	Rejected
)

type Order struct {
	Size   int
	Symbol Symbol
	Type   OrderType
	Status OrderStatus
}

// Broker interacts with a stock broker
type Broker interface {
	// SubmitOrder set an order and return a chan used to communicate the order status
	SubmitOrder(order Order) (<-chan Order, error)
}

type BacktestBrocker struct {
	InitialCashUSD float64
}

func (b *BacktestBrocker) SubmitOrder(order Order) (<-chan Order, error) {
	log.Fatal("NOT IMPLEMENTED")
	return nil, nil
}
