package gotrader

import (
	"context"
	"errors"
	"fmt"
	"golang.org/x/exp/slog"
	"math"
	"math/rand"
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
	ErrInvalidSize   = errors.New("order.size should be > 0")
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
	SubmitOrder(candle Candle, order Order) (string, error)
	GetOrderByID(OrderID string) (Order, error)
	ProcessOrders(candle Candle) []Order
	GetPosition(symbol Symbol) Position
	Shutdown()
	AvailableCash() float64
	ClosePosition(position Position) error
	GetPositions() []Position
}

// TickBroker is implemented by brokers that can process single trades and quotes
type TickBroker interface {
	// ProcessTrade fills the pending orders that are eligible at this trade. Returns the orders touched.
	ProcessTrade(trade Trade) []Order
	// ProcessQuote only advances the broker clock.
	ProcessQuote(quote Quote)
}

// DefaultBacktestLatency is the latency used by BacktestBrocker when Latency is not set
const DefaultBacktestLatency = 200 * time.Millisecond

type EvaluateCommissions func(order Order, price float64) float64

var Nocommissions = func(order Order, price float64) float64 { return 0 }

// BacktestBrocker is the default broker to back-test a strategy
type BacktestBrocker struct {
	BrokerAvailableCash float64
	OrderMap            map[string]*Order
	Portfolio           map[Symbol]Position
	EvalCommissions     EvaluateCommissions
	// Latency is the delay between the submission of an order and the first trade that can fill it
	Latency       time.Duration
	lastEventTime time.Time
	// Stdout              *log.Logger
	// Stderr              *log.Logger
	// Signals             Signal
}

func (b *BacktestBrocker) SubmitOrder(_ Candle, order Order) (string, error) {

	if order.Size <= 0 {
		return "", ErrInvalidSize
	}

	// Check that we do not have an open order for the same symbol
	var err error

	// This is disabled to add support for "invert position" and "doublebuy"
	// The strategy must submit multiple orders whithin the same candle
	// for existingOrderId, existingOrder := range b.OrderMap {
	// 	// existingOrder := o.(*Order)
	//
	// 	if existingOrder.Status >= OrderStatusFullFilled {
	// 		// No need to keep track of closed orders
	// 		delete(b.OrderMap, existingOrderId)
	// 		continue
	// 	}
	//
	// 	if existingOrder.Symbol != order.Symbol {
	// 		continue
	// 	}
	//
	// 	err = fmt.Errorf("order duplicated: the existing order %s is still open", existingOrderId)
	// 	break
	// }

	order.Id = RandUid()
	order.Status = OrderStatusAccepted

	if !b.lastEventTime.IsZero() {
		order.SubmittedTime = b.lastEventTime
	}

	if err != nil {
		order.Status = OrderStatusRejected
		return order.Id, err
	}

	b.OrderMap[order.Id] = &order
	return order.Id, err
}

func (b *BacktestBrocker) Shutdown() {
	b.OrderMap = nil
	b.Portfolio = nil

	b.OrderMap = map[string]*Order{}
	b.Portfolio = map[Symbol]Position{}
}

func (b *BacktestBrocker) GetOrderByID(orderID string) (Order, error) {
	order, found := b.OrderMap[orderID]
	if !found {
		return Order{}, ErrOrderNotFound
	}
	return *order, nil
}

func (b *BacktestBrocker) latency() time.Duration {
	if b.Latency == 0 {
		return DefaultBacktestLatency
	}
	return b.Latency
}

func (b *BacktestBrocker) ProcessOrders(candle Candle) []Order {
	ctx := GetNewContextFromCandle(candle)
	b.lastEventTime = candle.Time
	var orderPlaced []Order

	for _, order := range b.OrderMap {

		if order.SubmittedTime.IsZero() {
			order.SubmittedTime = candle.Time
		}

		if order.Status == OrderStatusFullFilled ||
			order.Status == OrderStatusRejected ||
			order.Symbol != candle.Symbol {
			continue
		}

		b.fill(order, candle.Open, candle.Time, ctx)
		orderPlaced = append(orderPlaced, *order)

	}

	return orderPlaced
}

// ProcessTrade fills the pending orders of the traded symbol that have been submitted
// at least Latency before this trade.
func (b *BacktestBrocker) ProcessTrade(trade Trade) []Order {
	b.lastEventTime = trade.Time()

	if !trade.UpdatesLast {
		return nil
	}

	ctx := GetNewContextFromCandle(Candle{Symbol: trade.Ticker, Time: trade.Time()})
	latency := b.latency()
	var orderPlaced []Order

	for _, order := range b.OrderMap {

		if order.Status == OrderStatusFullFilled ||
			order.Status == OrderStatusRejected ||
			order.Symbol != trade.Ticker {
			continue
		}

		if order.SubmittedTime.Add(latency).After(trade.Time()) {
			continue
		}

		b.fill(order, trade.Price, trade.Time(), ctx)
		orderPlaced = append(orderPlaced, *order)

	}

	return orderPlaced
}

func (b *BacktestBrocker) ProcessQuote(quote Quote) {
	b.lastEventTime = quote.Time()
}

func (b *BacktestBrocker) fill(order *Order, price float64, at time.Time, ctx context.Context) {
	atStr := fmt.Sprintf("%-5s %v", order.Symbol, at.Format("15:04:05"))

	order.Status = OrderStatusPartiallyFilled

	var orderQty int64

	// Order checks
	switch order.Type {
	case OrderBuy:
		orderQty = order.Size - order.SizeFilled

		// Check if the candle volume has room for our order
		// For testing purpose, we assume that our order is always processed
		// if orderQty > candle.Volume {
		// 	orderQty = candle.Volume
		// }

		// Do we have enough money to execute the order?
		requiredCash := float64(orderQty)*price + b.EvalCommissions(*order, price)
		if b.BrokerAvailableCash < requiredCash {
			slog.Error("order failed - no cash", "candle", atStr, "order", order.String(), "required", requiredCash, "available", b.BrokerAvailableCash)
		}

	case OrderSell:
		// Sell/short order are always executed
		// Use a negative size for sell orders, only for order management
		orderQty = -1 * order.Size
	default:
		panic("order type not supported")
	}

	// Execute the order!
	cashChange := math.Abs(float64(orderQty)) * price // SELL? orderQty is <0!
	oldPosition, haveInPortfolio := b.Portfolio[order.Symbol]
	newPosition := Position{
		Symbol:   order.Symbol,
		Size:     orderQty,
		AvgPrice: price,
	}
	order.AvgFilledPrice = price // <-- this is a bug. Need to calculate a weighted average

	// Update the available cash: use money to buy, add money if we are selling
	if orderQty > 0 { // || // BUY  -> use my cash
		// haveInPortfolio && orderQty < 0 { // CLOSE
		b.BrokerAvailableCash -= cashChange // cashChange is <0 is I'm selling
		MTradesBuy.Record(ctx, order.AvgFilledPrice)
	}

	if orderQty < 0 {
		b.BrokerAvailableCash += cashChange
		MTradesSell.Record(ctx, order.AvgFilledPrice)
	}

	// Update the Portfolio
	if haveInPortfolio {
		newPosition.Size += oldPosition.Size
		// warn: if I'm closing a position, newPosition.Size == +Inf
		// we don't care because the position is not added to the portfolio, but keep it in mind
		newPosition.AvgPrice = (float64(oldPosition.Size)*oldPosition.AvgPrice + float64(orderQty)*price) / float64(oldPosition.Size+orderQty)
	}

	// pl := 0.0
	if newPosition.Size == 0 {
		// the position has been closed; I can calculate the p&l for this trade
		// as the difference from the closing order and the position (for long)
		// pl = float64(order.Size)*order.AvgFilledPrice - float64(oldPosition.Size)*oldPosition.AvgPrice
		delete(b.Portfolio, order.Symbol)

		// // short order are on the opposite
		// if oldPosition.Size < 0 {
		// 	pl = -1*float64(oldPosition.Size)*oldPosition.AvgPrice - float64(order.Size)*order.AvgFilledPrice
		// }

	}

	if newPosition.Size != 0 {
		b.Portfolio[order.Symbol] = newPosition
	}

	// A trade is a position that has been opened and close;
	// try to get the final PL for the current trad
	// b.Signals.Append(candle, "trades_pl", pl)

	// Update the order status
	order.SizeFilled += int64(math.Abs(float64(orderQty)))
	if order.SizeFilled == order.Size {
		order.Status = OrderStatusFullFilled
	}

	slog.Info("order filled ", "candle_time", atStr, "order", order.String(), "qty", orderQty, "candle_open", price)
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

// ClosePosition @deprecated
func (b *BacktestBrocker) ClosePosition(position Position) error {
	var orderType OrderType

	if position.Size > 0 {
		orderType = OrderSell
	}
	if position.Size < 0 {
		orderType = OrderBuy
	}

	_, err := b.SubmitOrder(Candle{}, Order{
		Symbol: position.Symbol,
		Size:   int64(math.Abs(float64(position.Size))),
		Type:   orderType,
	})

	return err
}
