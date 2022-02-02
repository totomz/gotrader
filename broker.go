package gotrader

import (
	"errors"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"sync"
	"time"
)

type Symbol string

type OrderType int

const (
	OrderBuy = iota
	OrderSell
)

var (
	ErrOrderNotFound = errors.New("order not found")
)

type OrderStatus int

const (
	OrderStatusSubmitted = iota
	OrderStatusAccepted
	OrderStatusPartiallyFilled
	OrderStatusFullFilled
	OrderStatusRejected
)

type Position struct {
	// Size is negative if the position is a SHORT
	Size     int64
	AvgPrice float64
	Symbol   Symbol
}

type Order struct {
	Id string
	// Size is always > 0
	Size   int64
	Symbol Symbol
	Type   OrderType
	Status OrderStatus
	// SizeFilled is always > 0
	SizeFilled     int64
	AvgFilledPrice float64

	// SubmittedTime When the order has been submitted (candle time)
	SubmittedTime time.Time
}

func (o Order) String() string {
	var orderType string
	switch t := o.Type; t {
	case OrderBuy:
		orderType = "BUY"
	case OrderSell:
		orderType = "SELL"
	default:
		orderType = "BOH"
	}

	return fmt.Sprintf("{ [%s]: %5s %v %v }", o.Id, orderType, o.Size, o.Symbol)
}

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func RandUid() string {
	n := 6
	a := make([]byte, n)
	b := make([]byte, n)
	// c := make([]byte, n)

	for i := range b {
		a[i] = letterBytes[rand.Intn(len(letterBytes))]
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
		// c[i] = letterBytes[rand.Intn(len(letterBytes))]
	}

	return string(a) + "-" + string(b) // + "-" + string(c)
}

// Broker interacts with a stock broker
type Broker interface {
	SubmitOrder(order Order) (string, error)
	GetOrderByID(OrderID string) (Order, error)
	ProcessOrders(candle Candle) []Order
	GetPosition(symbol Symbol) Position
	Shutdown()
	AvailableCash() float64
	ClosePosition(position Position) error
	GetPositions() []Position
}

type EvaluateCommissions func(order Order, price float64) float64

var Nocommissions = func(order Order, price float64) float64 { return 0 }

// BacktestBrocker is the default broker to back-test a strategy
type BacktestBrocker struct {
	InitialCashUSD      float64
	BrokerAvailableCash float64
	OrderMap            sync.Map
	Portfolio           map[Symbol]Position
	EvalCommissions     EvaluateCommissions
	Stdout              *log.Logger
	Stderr              *log.Logger
}

func (b *BacktestBrocker) SubmitOrder(order Order) (string, error) {

	if order.Size <= 0 {
		return "", errors.New("order size must be > 0")
	}

	order.Id = RandUid()
	order.Status = OrderStatusAccepted

	b.OrderMap.Store(order.Id, &order)

	return order.Id, nil
}

func (b *BacktestBrocker) Shutdown() {

}

func (b *BacktestBrocker) GetOrderByID(orderID string) (Order, error) {
	order, found := b.OrderMap.Load(orderID)
	if !found {
		return Order{}, ErrOrderNotFound
	}
	return *order.(*Order), nil
}

func (b *BacktestBrocker) ProcessOrders(candle Candle) []Order {
	if b.Stdout == nil {
		b.Stdout = log.New(os.Stdout, "", log.Lshortfile|log.Ltime)
	}
	if b.Stderr == nil {
		b.Stdout = log.New(os.Stdout, "", log.Lshortfile|log.Ltime)
	}

	// b.Stdout.Printf(fmt.Sprintf("[%v] processing orders ", candle.TimeStr()))
	var orderPlaced []Order

	b.OrderMap.Range(func(key interface{}, value interface{}) bool {

		order := value.(*Order)

		if order.SubmittedTime.IsZero() {
			order.SubmittedTime = candle.Time
		}

		if order.Status == OrderStatusFullFilled ||
			order.Status == OrderStatusRejected ||
			order.Symbol != candle.Symbol {
			// b.Stdout.Printf(".    --> %s SKIPPED", order.String())
			return true
		}

		// b.Stdout.Printf("[%s]    --> %s ", candle.TimeStr(), order.String())
		order.Status = OrderStatusPartiallyFilled

		var orderQty int64

		// Order checks
		switch order.Type {
		case OrderBuy:
			orderQty = order.Size - order.SizeFilled
			if candle.Volume == 0 {
				return true
			}

			// Check if the candle volume has room for our order
			// For testing purpose, we assume that our order is always processed
			if orderQty > candle.Volume {
				orderQty = candle.Volume
			}

			// Do we have enough money to execute the order?
			requiredCash := float64(orderQty)*candle.Open + b.EvalCommissions(*order, candle.Open)
			if b.BrokerAvailableCash < requiredCash {
				b.Stderr.Fatalf("[%s]    --> %s - order failed - no cash, need $%v have $%v", candle.TimeStr(), order.String(), requiredCash, b.BrokerAvailableCash)
				return true
			}

		case OrderSell:
			// Sell/short order are always executed
			// Use a negative size for sell orders, only for order management
			orderQty = -1 * order.Size
		default:
			panic("order type not supported")
		}

		// Execute the order!
		cashChange := math.Abs(float64(orderQty)) * candle.Open // SELL? orderQty is <0!
		oldPosition, haveInPortfolio := b.Portfolio[order.Symbol]
		newPosition := Position{
			Symbol:   order.Symbol,
			Size:     orderQty,
			AvgPrice: candle.Open,
		}

		// Update the available cahs: use money to buy, add money if we are selling
		if orderQty > 0 { // || // BUY  -> use my cash
			// haveInPortfolio && orderQty < 0 { // CLOSE
			b.BrokerAvailableCash -= cashChange // cashChange is <0 is I'm selling
		}

		if orderQty < 0 {
			b.BrokerAvailableCash += cashChange
		}

		// Update the Portfolio
		if haveInPortfolio {
			newPosition.Size += oldPosition.Size
			// warn: if I'm closing a position, newPosition.Size == +Inf
			// we don't care because the position is not added to the portfolio, but keep it in mind
			newPosition.AvgPrice = (float64(oldPosition.Size)*oldPosition.AvgPrice + float64(orderQty)*candle.Open) / float64(oldPosition.Size+orderQty)
		}

		if newPosition.Size == 0 {
			delete(b.Portfolio, order.Symbol)
		} else {
			b.Portfolio[order.Symbol] = newPosition
		}

		// Update the order status
		order.SizeFilled += int64(math.Abs(float64(orderQty)))
		order.AvgFilledPrice = candle.Open // <-- this is a bug. Need to calculate a weighted average
		if order.SizeFilled == order.Size {
			order.Status = OrderStatusFullFilled
		}

		b.Stdout.Printf("[%s]    --> %s: filled %v@%v ", candle.TimeStr(), order.String(), orderQty, candle.Open)
		orderPlaced = append(orderPlaced, *order)

		return true
	})

	return orderPlaced
}

func (b *BacktestBrocker) AvailableCash() float64 {
	return b.BrokerAvailableCash
}

func (b *BacktestBrocker) GetPosition(symbol Symbol) Position {
	position := b.Portfolio[symbol]
	return position
}

func (b *BacktestBrocker) GetPositions() []Position {
	var openPositions []Position
	for _, v := range b.Portfolio {
		openPositions = append(openPositions, v)
	}
	return openPositions
}

func (b *BacktestBrocker) ClosePosition(position Position) error {
	var orderType OrderType

	if position.Size > 0 {
		orderType = OrderSell
	}
	if position.Size < 0 {
		orderType = OrderBuy
	}

	b.SubmitOrder(Order{
		Symbol: position.Symbol,
		Size:   int64(math.Abs(float64(position.Size))),
		Type:   orderType,
	})

	return nil
}
