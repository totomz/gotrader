package massive

import (
	"fmt"
	"math"
	"time"

	"github.com/massive-com/client-go/v3/websocket/models"
	"github.com/totomz/gotrader"
)

// eventFromMessage turns one frame of the vendor output channel into a
// framework MarketEvent. Only equity trades and quotes are mapped: anything
// else is an input error, which the caller counts, logs and skips.
func eventFromMessage(msg any) (gotrader.MarketEvent, error) {
	switch frame := msg.(type) {
	case models.EquityTrade:
		trade, err := tradeFromMassive(&frame)
		if err != nil {
			return gotrader.MarketEvent{}, err
		}
		return gotrader.MarketEvent{Trade: &trade}, nil

	case models.EquityQuote:
		quote, err := quoteFromMassive(&frame)
		if err != nil {
			return gotrader.MarketEvent{}, err
		}
		return gotrader.MarketEvent{Quote: &quote}, nil
	}

	return gotrader.MarketEvent{}, fmt.Errorf("massive: unexpected frame of type %T", msg)
}

// tradeFromMassive maps a vendor trade onto the framework type.
// The vendor fields i, trfi and trft are dropped on purpose: see analysis.md
// §3 out-of-scope-vendor-fields. ParticipantTS, UpdatesLast and UpdatesVolume
// are not carried by the websocket feed and stay at their zero value
// (analysis.md OP-1). IsRTH is not carried either and is derived here from the
// trade timestamp.
func tradeFromMassive(msg *models.EquityTrade) (gotrader.Trade, error) {
	if msg == nil || msg.Symbol == "" {
		return gotrader.Trade{}, fmt.Errorf("massive: trade frame without a symbol")
	}

	exchange, err := narrowInt16(msg.Exchange, "trade exchange")
	if err != nil {
		return gotrader.Trade{}, err
	}

	tape, err := narrowInt8(msg.Tape, "trade tape")
	if err != nil {
		return gotrader.Trade{}, err
	}

	conditions, err := narrowInt16Slice(msg.Conditions, "trade condition")
	if err != nil {
		return gotrader.Trade{}, err
	}

	return gotrader.Trade{
		Ticker:     gotrader.Symbol(msg.Symbol),
		TS:         msg.Timestamp,
		Seq:        msg.SequenceNumber,
		Price:      msg.Price,
		Size:       msg.Size,
		Exchange:   exchange,
		Tape:       tape,
		Conditions: conditions,
		IsRTH:      gotrader.IsNasdaqTradingTime(time.UnixMilli(msg.Timestamp).UTC()),
	}, nil
}

// quoteFromMassive maps a vendor quote onto the framework type. The vendor
// sizes are round lots on the wire and are carried as they are: this package
// does not interpret them.
func quoteFromMassive(msg *models.EquityQuote) (gotrader.Quote, error) {
	if msg == nil || msg.Symbol == "" {
		return gotrader.Quote{}, fmt.Errorf("massive: quote frame without a symbol")
	}

	bidExchange, err := narrowInt16(msg.BidExchangeID, "quote bid exchange")
	if err != nil {
		return gotrader.Quote{}, err
	}

	askExchange, err := narrowInt16(msg.AskExchangeID, "quote ask exchange")
	if err != nil {
		return gotrader.Quote{}, err
	}

	condition, err := narrowInt16(msg.Condition, "quote condition")
	if err != nil {
		return gotrader.Quote{}, err
	}

	tape, err := narrowInt8(msg.Tape, "quote tape")
	if err != nil {
		return gotrader.Quote{}, err
	}

	indicators, err := narrowInt16Slice(msg.Indicators, "quote indicator")
	if err != nil {
		return gotrader.Quote{}, err
	}

	return gotrader.Quote{
		Ticker:      gotrader.Symbol(msg.Symbol),
		TS:          msg.Timestamp,
		Seq:         msg.SequenceNumber,
		BidPrice:    msg.BidPrice,
		AskPrice:    msg.AskPrice,
		BidSize:     int64(msg.BidSize),
		AskSize:     int64(msg.AskSize),
		BidExchange: bidExchange,
		AskExchange: askExchange,
		Condition:   condition,
		Indicators:  indicators,
		Tape:        tape,
	}, nil
}

// narrowInt16 converts a vendor int32 to the framework int16, refusing the
// conversion when the value does not fit: a silent truncation here would
// corrupt every consumer downstream.
func narrowInt16(value int32, field string) (int16, error) {
	if value < math.MinInt16 || value > math.MaxInt16 {
		return 0, fmt.Errorf("massive: %s %d does not fit in an int16", field, value)
	}
	return int16(value), nil
}

// narrowInt8 converts a vendor int32 to the framework int8, refusing the
// conversion when the value does not fit.
func narrowInt8(value int32, field string) (int8, error) {
	if value < math.MinInt8 || value > math.MaxInt8 {
		return 0, fmt.Errorf("massive: %s %d does not fit in an int8", field, value)
	}
	return int8(value), nil
}

// narrowInt16Slice converts a vendor int32 slice to the framework int16 one.
// An empty source leaves the destination nil, like tradeFromParquetRow does.
func narrowInt16Slice(values []int32, field string) ([]int16, error) {
	if len(values) == 0 {
		return nil, nil
	}

	narrowed := make([]int16, len(values))
	for i, value := range values {
		narrow, err := narrowInt16(value, field)
		if err != nil {
			return nil, err
		}
		narrowed[i] = narrow
	}

	return narrowed, nil
}
