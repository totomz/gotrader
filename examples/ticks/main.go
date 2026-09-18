package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	"github.com/totomz/gotrader"
	"golang.org/x/exp/slog"
)

const (
	fixtureEventsPerSymbol = 25
	fixtureIntervalMs      = int64(1000)
	fixtureOpenOffset      = 14*time.Hour + 30*time.Minute
	startingCash           = 30000.0
	orderSize              = int64(1)
)

type TickStrategy struct {
	broker        gotrader.Broker
	lastQuote     map[gotrader.Symbol]gotrader.Quote
	invalidQuotes int
	trades        int
	closedCandles int
	entrySymbol   gotrader.Symbol
	exitSubmitted bool
	orderIds      []string
}

func (s *TickStrategy) Initialize(cerbero *gotrader.Cerbero) {
	s.broker = cerbero.Broker
	s.lastQuote = map[gotrader.Symbol]gotrader.Quote{}
}

func (s *TickStrategy) OnTrade(_ gotrader.Trade) {
	s.trades++
}

func (s *TickStrategy) OnQuote(quote gotrader.Quote) {
	if !quote.IsValid() {
		s.invalidQuotes++
		return
	}

	s.lastQuote[quote.Ticker] = quote
}

func (s *TickStrategy) Eval(candles []gotrader.Candle) {
	candle := candles[len(candles)-1]
	quote := s.lastQuote[candle.Symbol]
	s.closedCandles++

	slog.Info("candle closed",
		"symbol", candle.Symbol, "time", candle.Time.Format(time.RFC3339),
		"open", candle.Open, "high", candle.High, "low", candle.Low, "close", candle.Close,
		"volume", candle.Volume, "last_mid", quote.Mid(), "last_quote_ts", quote.TS)

	if s.entrySymbol == "" {
		s.entrySymbol = candle.Symbol
		s.submitOrder(candle, gotrader.OrderBuy)
		return
	}

	if s.exitSubmitted || candle.Symbol != s.entrySymbol {
		return
	}

	s.exitSubmitted = true
	s.submitOrder(candle, gotrader.OrderSell)
}

func (s *TickStrategy) submitOrder(candle gotrader.Candle, orderType gotrader.OrderType) {
	orderId, err := s.broker.SubmitOrder(candle, gotrader.Order{
		Size:   orderSize,
		Symbol: candle.Symbol,
		Type:   orderType,
	})

	if err != nil {
		slog.Error("can not submit the order", "symbol", candle.Symbol, "type", orderTypeName(orderType), "error", err)
		return
	}

	s.orderIds = append(s.orderIds, orderId)
	slog.Info("order submitted", "order_id", orderId, "symbol", candle.Symbol, "type", orderTypeName(orderType), "size", orderSize)
}

func (s *TickStrategy) Shutdown() {
	slog.Info("strategy shutdown", "invalid_quotes", s.invalidQuotes, "trades", s.trades, "closed_candles", s.closedCandles)

	for _, orderId := range s.orderIds {
		order, err := s.broker.GetOrderByID(orderId)
		if err != nil {
			slog.Error("can not read the order", "order_id", orderId, "error", err)
			continue
		}

		slog.Info("fill", "order_id", order.Id, "symbol", order.Symbol, "type", orderTypeName(order.Type),
			"size_filled", order.SizeFilled, "avg_filled_price", order.AvgFilledPrice, "status", order.Status)
	}

	slog.Info("final position", "symbol", s.entrySymbol, "size", s.broker.GetPosition(s.entrySymbol).Size)
}

func orderTypeName(orderType gotrader.OrderType) string {
	if orderType == gotrader.OrderBuy {
		return "BUY"
	}
	return "SELL"
}

func writeFixture(dataFolder string, day time.Time, symbols []gotrader.Symbol) error {
	baseTS := day.Add(fixtureOpenOffset).UnixMilli()

	for i, symbol := range symbols {
		basePrice := 100 + float64(i)*50
		trades := make([]gotrader.Trade, 0, fixtureEventsPerSymbol)
		quotes := make([]gotrader.Quote, 0, fixtureEventsPerSymbol)

		for j := 0; j < fixtureEventsPerSymbol; j++ {
			ts := baseTS + int64(j)*fixtureIntervalMs
			price := math.Round((basePrice+float64(j)*0.01)*100) / 100

			trades = append(trades, gotrader.Trade{
				Ticker:        symbol,
				TS:            ts,
				ParticipantTS: ts,
				Seq:           int64(j + 1),
				Price:         price,
				Size:          100,
				Exchange:      4,
				Tape:          3,
				UpdatesLast:   true,
				UpdatesVolume: true,
				IsRTH:         true,
			})

			quotes = append(quotes, fixtureQuote(symbol, ts+fixtureIntervalMs/2, int64(j+1), price, j))
		}

		if err := gotrader.WriteParquetTrades(dataFolder, day, symbol, trades); err != nil {
			return fmt.Errorf("can not write the trades fixture of %s: %w", symbol, err)
		}

		if err := gotrader.WriteParquetQuotes(dataFolder, day, symbol, quotes); err != nil {
			return fmt.Errorf("can not write the quotes fixture of %s: %w", symbol, err)
		}
	}

	return nil
}

// fixtureQuote builds a quote around price; the indexes 3, 7 and 11 are invalid quotes:
// a crossed one, one without bid and one without sizes.
func fixtureQuote(symbol gotrader.Symbol, ts, seq int64, price float64, index int) gotrader.Quote {
	quote := gotrader.Quote{
		Ticker:      symbol,
		TS:          ts,
		Seq:         seq,
		BidPrice:    math.Round((price-0.01)*100) / 100,
		AskPrice:    math.Round((price+0.01)*100) / 100,
		BidSize:     200,
		AskSize:     300,
		BidExchange: 11,
		AskExchange: 12,
		Tape:        3,
	}

	switch index {
	case 3:
		quote.BidPrice, quote.AskPrice = quote.AskPrice, quote.BidPrice
	case 7:
		quote.BidPrice = 0
	case 11:
		quote.BidSize = 0
		quote.AskSize = 0
	}

	return quote
}

func main() {
	dataFolder := flag.String("data", "", "folder holding the parquet trades and quotes")
	day := flag.String("day", "", "day to backtest, YYYY-MM-DD")
	symbols := flag.String("symbols", "", "comma separated list of symbols")
	fixture := flag.Bool("fixture", false, "write a synthetic parquet fixture in the data folder before running")
	flag.Parse()

	sday, err := time.Parse("2006-01-02", *day)
	if err != nil {
		slog.Error("can not parse the day", "day", *day, "error", err)
		os.Exit(1)
	}

	var tickers []gotrader.Symbol
	for _, symbol := range strings.Split(*symbols, ",") {
		symbol = strings.TrimSpace(symbol)
		if symbol == "" {
			continue
		}
		tickers = append(tickers, gotrader.Symbol(symbol))
	}

	if len(tickers) == 0 {
		slog.Error("no symbol to backtest", "symbols", *symbols)
		os.Exit(1)
	}

	if *fixture {
		if err = writeFixture(*dataFolder, sday, tickers); err != nil {
			slog.Error("can not write the fixture", "data", *dataFolder, "error", err)
			os.Exit(1)
		}
		slog.Info("fixture written", "data", *dataFolder, "day", sday.Format("2006-01-02"), "symbols", *symbols)
	}

	service := gotrader.Cerbero{
		Broker: &gotrader.BacktestBrocker{
			BrokerAvailableCash: startingCash,
			OrderMap:            map[string]*gotrader.Order{},
			Portfolio:           map[gotrader.Symbol]gotrader.Position{},
			EvalCommissions:     gotrader.Nocommissions,
		},
		Strategy: &TickStrategy{},
		TickFeed: &gotrader.GenericParquetTrades{
			DataFolder: *dataFolder,
			Day:        sday,
			Symbols:    tickers,
		},
	}

	result, err := service.Run()
	if err != nil {
		slog.Error("the backtest failed", "error", err)
		os.Exit(1)
	}

	slog.Info("execution result", "total_time", result.TotalTimeString, "initial_cash", result.InitialCash,
		"final_cash", result.FinalCash, "pl", result.PL)
}
