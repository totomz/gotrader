package alpacabroker

import (
	"fmt"
	"github.com/alpacahq/alpaca-trade-api-go/v2/alpaca"
	"github.com/shopspring/decimal"
	"github.com/totomz/gotrader"
	"log"
	"time"
)

type AlpacaBroker struct {
	Stdout *log.Logger
	Stderr *log.Logger
	// Signals gotrader.Signal
	client alpaca.Client
}

func NewAlpacaBroker(config AlpacaBroker, apiKey, apiSecret, baseUrl string) *AlpacaBroker {

	client := alpaca.NewClient(alpaca.ClientOpts{
		ApiKey:    apiKey,
		ApiSecret: apiSecret,
		BaseURL:   baseUrl,
	})
	config.client = client

	return &config
}

func (ab *AlpacaBroker) SignalsPortfolioStatus() {
	panic("TODO")
	// positions, err := ab.client.ListPositions()
	// if err != nil {
	// 	ab.Stderr.Printf("polling can't list positions: %v", err)
	// }
	//
	// totalAssets := ab.AvailableCash()

	// for _, p := range positions {
	// 	c := gotrader.Candle{
	// 		Symbol: gotrader.Symbol(p.Symbol),
	// 		Time:   time.Now(),
	// 	}
	// 	pl := p.UnrealizedPL.InexactFloat64()
	// 	totalAssets += pl
	// 	ab.Signals.Append(c, "alpaca.stock.pl", pl)
	// 	ab.Signals.Append(c, "alpaca.stock.qty", p.Qty.InexactFloat64())
	// }
	//
	// // signals sucks, why am I passing a Candle?
	// c := gotrader.Candle{
	// 	Symbol: "AMD",
	// 	Time:   time.Now(),
	// }
	// ab.Signals.Append(c, "alpaca.totalAssets", totalAssets)
}

func (ab *AlpacaBroker) Shutdown() {
	// do nothing
}

func (ab *AlpacaBroker) ProcessOrders(candle gotrader.Candle) []gotrader.Order {
	// This method is only required for backtesting, to process the orders at new candles.
	return nil
}

func (ab *AlpacaBroker) AvailableCash() float64 {

	account, err := ab.client.GetAccount()
	if err != nil {
		ab.Stderr.Panic(err)
	}

	if account.Status != "ACTIVE" {
		return 0
	}

	return account.Cash.InexactFloat64()
}

func OrderToString(order *alpaca.Order) string {
	return fmt.Sprintf("{%s - %s %v %s }", order.ID, order.Side, order.Qty, order.Symbol)
}

func (ab *AlpacaBroker) SubmitOrder(candle gotrader.Candle, order gotrader.Order) (string, error) {

	// if ab.DisableOrders {
	// 	ab.Stderr.Printf("alpaca orders are disabled!")
	// 	return "", nil
	// }
	symbl := string(order.Symbol)
	qty := decimal.NewFromInt(order.Size)
	side := "buy"
	sizeSide := float64(order.Size)

	if order.Type == gotrader.OrderSell {
		side = "sell"
		sizeSide *= -1
	}

	orderRequest := alpaca.PlaceOrderRequest{
		AssetKey:    &symbl,
		Qty:         &qty,
		Side:        alpaca.Side(side),
		Type:        "market",
		TimeInForce: "day",
	}
	placedOrder, err := ab.client.PlaceOrder(orderRequest)
	if err != nil {
		ab.Stderr.Printf("can't place order %s: %v", order.String(), err)
		return "", err
	}

	ab.Stdout.Printf("submitted order %s", OrderToString(placedOrder))

	// The order is submitted but we don't know yet the
	// avgFlledPrice, neither if it has been fullfiled or not.
	panic("TODO")
	// ab.Signals.Append(candle, fmt.Sprintf("trades_%s", side), candle.Close)
	// ab.Signals.Append(candle, "trades_size", sizeSide)

	return placedOrder.ID, nil
}

func (ab *AlpacaBroker) GetOrderByID(OrderID string) (gotrader.Order, error) {
	order, err := ab.client.GetOrder(OrderID)
	if err != nil {
		return gotrader.Order{}, err
	}

	avgFilledSize := 0.0

	if order.FilledAvgPrice != nil {
		avgFilledSize = order.FilledAvgPrice.InexactFloat64()
	}

	o := gotrader.Order{
		Id:             order.ID,
		Size:           order.FilledQty.IntPart(),
		Symbol:         gotrader.Symbol(order.Symbol),
		SizeFilled:     order.FilledQty.IntPart(),
		AvgFilledPrice: avgFilledSize,
		SubmittedTime:  order.SubmittedAt,
	}

	switch order.Status {
	case "partially_filled":
		o.Status = gotrader.OrderStatusPartiallyFilled
	case "filled":
		o.Status = gotrader.OrderStatusFullFilled
	case "cancelled":
		o.Status = gotrader.OrderStatusRejected
	default:
		o.Status = gotrader.OrderStatusAccepted
	}

	if order.Side == "sell" {
		o.Type = gotrader.OrderSell
	}

	return o, nil
}

func (ab *AlpacaBroker) GetPosition(symbol gotrader.Symbol) gotrader.Position {
	zeroVal := gotrader.Position{
		Size:     0,
		AvgPrice: 0,
		Symbol:   symbol,
	}

	pos, err := ab.client.GetPosition(string(symbol))
	if err != nil {
		if err.Error() == "position does not exist" {
			return zeroVal
		}

		ab.Stderr.Printf("error getting position for %v: %v", symbol, err)
		return zeroVal
	}

	return PositionMap(pos)
}

func (ab *AlpacaBroker) ClosePosition(position gotrader.Position) error {
	symbol := string(position.Symbol)

	err := ab.client.ClosePosition(symbol)
	if err != nil {
		return err
	}

	for {
		p, e := ab.client.GetPosition(symbol)
		if e != nil {
			if e.Error() == "position does not exist" {
				break
			}
			return e
		}

		if p.Qty.IsZero() {
			break
		}

		time.Sleep(100 * time.Millisecond)
	}

	return err
}

func (ab *AlpacaBroker) GetPositions() []gotrader.Position {
	var zeroVal []gotrader.Position
	positions, err := ab.client.ListPositions()
	if err != nil {
		ab.Stderr.Printf("error getting positions %v", err)
		return zeroVal
	}

	var pos []gotrader.Position

	for _, p := range positions {
		pos = append(pos, PositionMap(&p))
	}

	return pos
}

func PositionMap(input *alpaca.Position) gotrader.Position {
	return gotrader.Position{
		Size:     input.Qty.IntPart(),
		AvgPrice: input.EntryPrice.InexactFloat64(),
		Symbol:   gotrader.Symbol(input.Symbol),
	}
}
