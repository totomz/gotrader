package alpacabroker

import (
	"context"
	"fmt"
	"github.com/alpacahq/alpaca-trade-api-go/v2/alpaca"
	"github.com/shopspring/decimal"
	"github.com/totomz/gotrader"
	"log/slog"
	"time"
)

type AlpacaBroker struct {
	client alpaca.Client
}

var (
	MAlpacaPl     = gotrader.NewMetricWithDefaultViews("alpaca/stock/pl")
	MAlpacaQty    = gotrader.NewMetricWithDefaultViews("alpaca/stock/qty")
	MAlpacaAssets = gotrader.NewMetricWithDefaultViews("alpaca/total_assets")
)

func NewAlpacaBroker(apiKey, apiSecret, baseUrl string) *AlpacaBroker {

	client := alpaca.NewClient(alpaca.ClientOpts{
		ApiKey:    apiKey,
		ApiSecret: apiSecret,
		BaseURL:   baseUrl,
	})

	return &AlpacaBroker{
		client: client,
	}
}

func (ab *AlpacaBroker) SignalsPortfolioStatus() {
	positions, err := ab.client.ListPositions()
	if err != nil {
		slog.Error("polling can't list positions", "error", err)
	}
	ctx := context.Background()
	totalAssets := ab.AvailableCash()

	for _, p := range positions {
		pl := p.UnrealizedPL.InexactFloat64()
		totalAssets += pl
		MAlpacaPl.Record(ctx, pl)
		MAlpacaQty.Record(ctx, p.Qty.InexactFloat64())
	}

	MAlpacaAssets.Record(ctx, totalAssets)
}

func (ab *AlpacaBroker) Shutdown() {
	// do nothing
}

func (ab *AlpacaBroker) ProcessOrders(_ gotrader.Candle) []gotrader.Order {
	// This method is only required for backtesting, to process the orders at new candles.
	return nil
}

func (ab *AlpacaBroker) AvailableCash() float64 {

	account, err := ab.client.GetAccount()
	if err != nil {
		slog.Error("can't get account", "error", err)
		return 0
	}

	if account.Status != "ACTIVE" {
		return 0
	}

	return account.Cash.InexactFloat64()
}

func OrderToString(order *alpaca.Order) string {
	return fmt.Sprintf("{%s - %s %v %s }", order.ID, order.Side, order.Qty, order.Symbol)
}

func (ab *AlpacaBroker) SubmitOrder(_ gotrader.Candle, order gotrader.Order) (string, error) {

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
		slog.Error("can't place order", "order", order.String(), "error", err, "symbol", order.Symbol)
		return "", err
	}

	slog.Info("submitted order", "order", OrderToString(placedOrder), "symbol", order.Symbol)

	// The order is submitted, but we don't know yet the
	// avgFlledPrice, neither if it has been fullfiled or not.

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

		slog.Error("error getting position for %v: %v", symbol, err)
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
		slog.Error("error getting positions", "error", err)
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
