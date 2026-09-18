package gotrader

import (
	"fmt"
	"time"
)

type Trade struct {
	Ticker        Symbol
	TS            int64
	ParticipantTS int64
	Seq           int64
	Price         float64
	Size          int64
	Exchange      int16
	Tape          int8
	Conditions    []int16
	UpdatesLast   bool
	UpdatesVolume bool
	IsRTH         bool
}

func (t Trade) Time() time.Time {
	return time.UnixMilli(t.TS).UTC()
}

func (t Trade) String() string {
	return fmt.Sprintf("[%-5s %v] price:%v size:%v exchange:%v tape:%v seq:%v updates_last:%v updates_volume:%v is_rth:%v",
		t.Ticker, t.Time().Format("15:04:05.000"), t.Price, t.Size, t.Exchange, t.Tape, t.Seq, t.UpdatesLast, t.UpdatesVolume, t.IsRTH)
}

type Quote struct {
	Ticker      Symbol
	TS          int64
	Seq         int64
	BidPrice    float64
	AskPrice    float64
	BidSize     int64
	AskSize     int64
	BidExchange int16
	AskExchange int16
	Condition   int16
	Indicators  []int16
	Tape        int8
}

func (q Quote) Time() time.Time {
	return time.UnixMilli(q.TS).UTC()
}

func (q Quote) Mid() float64 {
	return (q.BidPrice + q.AskPrice) / 2
}

func (q Quote) Spread() float64 {
	return q.AskPrice - q.BidPrice
}

func (q Quote) SpreadRel() float64 {
	mid := q.Mid()
	if mid == 0 {
		return 0
	}
	return q.Spread() / mid
}

func (q Quote) IsValid() bool {
	if q.BidPrice <= 0 || q.AskPrice <= 0 {
		return false
	}
	if q.AskPrice < q.BidPrice {
		return false
	}
	if q.BidSize == 0 && q.AskSize == 0 {
		return false
	}
	return true
}

func (q Quote) String() string {
	return fmt.Sprintf("[%-5s %v] bid:%v x %v ask:%v x %v spread:%v seq:%v valid:%v",
		q.Ticker, q.Time().Format("15:04:05.000"), q.BidPrice, q.BidSize, q.AskPrice, q.AskSize, q.Spread(), q.Seq, q.IsValid())
}

type MarketEvent struct {
	Trade *Trade
	Quote *Quote
}

func (e MarketEvent) IsTrade() bool {
	return e.Trade != nil
}

func (e MarketEvent) IsQuote() bool {
	return e.Quote != nil
}

func (e MarketEvent) TS() int64 {
	if e.Trade != nil {
		return e.Trade.TS
	}
	if e.Quote != nil {
		return e.Quote.TS
	}
	return 0
}

func (e MarketEvent) Symbol() Symbol {
	if e.Trade != nil {
		return e.Trade.Ticker
	}
	if e.Quote != nil {
		return e.Quote.Ticker
	}
	return Symbol("")
}
