package massive

import (
	"math"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/massive-com/client-go/v3/websocket/models"
	"github.com/totomz/gotrader"
)

func TestMapping(t *testing.T) {
	t.Parallel()

	t.Run("trade carries every mapped field", func(t *testing.T) {
		msg := models.EquityTrade{
			Symbol:                          "AAPL",
			Exchange:                        11,
			ID:                              "dropped",
			Tape:                            3,
			Price:                           241.35,
			Size:                            120,
			Conditions:                      []int32{12, 37},
			Timestamp:                       1733914800123,
			SequenceNumber:                  987654,
			TradeReportingFacilityID:        2,
			TradeReportingFacilityTimestamp: 1733914800200,
		}

		got, err := tradeFromMassive(&msg)
		if err != nil {
			t.Fatalf("tradeFromMassive() error = %v", err)
		}

		want := gotrader.Trade{
			Ticker:     gotrader.Symbol("AAPL"),
			TS:         1733914800123,
			Seq:        987654,
			Price:      241.35,
			Size:       120,
			Exchange:   11,
			Tape:       3,
			Conditions: []int16{12, 37},
		}

		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("unexpected trade (-want +got):\n%s", diff)
		}
	})

	t.Run("a trade without conditions leaves the slice nil", func(t *testing.T) {
		msg := models.EquityTrade{Symbol: "AMZN", Timestamp: 1, Price: 2, Size: 3}

		got, err := tradeFromMassive(&msg)
		if err != nil {
			t.Fatalf("tradeFromMassive() error = %v", err)
		}

		if got.Conditions != nil {
			t.Errorf("Conditions = %v, want nil", got.Conditions)
		}
	})

	t.Run("quote carries every mapped field", func(t *testing.T) {
		msg := models.EquityQuote{
			Symbol:         "AMZN",
			BidExchangeID:  8,
			BidPrice:       226.11,
			BidSize:        4,
			AskExchangeID:  12,
			AskPrice:       226.19,
			AskSize:        7,
			Condition:      1,
			Indicators:     []int32{4, 9},
			Timestamp:      1733914800456,
			Tape:           2,
			SequenceNumber: 123456,
		}

		got, err := quoteFromMassive(&msg)
		if err != nil {
			t.Fatalf("quoteFromMassive() error = %v", err)
		}

		want := gotrader.Quote{
			Ticker:      gotrader.Symbol("AMZN"),
			TS:          1733914800456,
			Seq:         123456,
			BidPrice:    226.11,
			AskPrice:    226.19,
			BidSize:     4,
			AskSize:     7,
			BidExchange: 8,
			AskExchange: 12,
			Condition:   1,
			Indicators:  []int16{4, 9},
			Tape:        2,
		}

		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("unexpected quote (-want +got):\n%s", diff)
		}
	})

	t.Run("a trade frame becomes a trade market event", func(t *testing.T) {
		event, err := eventFromMessage(models.EquityTrade{Symbol: "AAPL", Timestamp: 7, Price: 1})
		if err != nil {
			t.Fatalf("eventFromMessage() error = %v", err)
		}

		if !event.IsTrade() || event.Symbol() != gotrader.Symbol("AAPL") || event.TS() != 7 {
			t.Errorf("unexpected event %+v", event)
		}
	})

	t.Run("a quote frame becomes a quote market event", func(t *testing.T) {
		event, err := eventFromMessage(models.EquityQuote{Symbol: "AAPL", Timestamp: 9, BidPrice: 1})
		if err != nil {
			t.Fatalf("eventFromMessage() error = %v", err)
		}

		if !event.IsQuote() || event.Symbol() != gotrader.Symbol("AAPL") || event.TS() != 9 {
			t.Errorf("unexpected event %+v", event)
		}
	})
}

func TestMappingErrors(t *testing.T) {
	t.Parallel()

	t.Run("a nil or symbol-less frame fails", func(t *testing.T) {
		if _, err := tradeFromMassive(nil); err == nil {
			t.Error("tradeFromMassive(nil) should fail")
		}

		if _, err := quoteFromMassive(nil); err == nil {
			t.Error("quoteFromMassive(nil) should fail")
		}

		if _, err := tradeFromMassive(&models.EquityTrade{Timestamp: 1}); err == nil {
			t.Error("a trade without a symbol should fail")
		}

		if _, err := quoteFromMassive(&models.EquityQuote{Timestamp: 1}); err == nil {
			t.Error("a quote without a symbol should fail")
		}
	})

	t.Run("an out of range value fails instead of truncating", func(t *testing.T) {
		cases := map[string]func() error{
			"trade exchange": func() error {
				_, err := tradeFromMassive(&models.EquityTrade{Symbol: "AAPL", Exchange: math.MaxInt16 + 1})
				return err
			},
			"trade tape": func() error {
				_, err := tradeFromMassive(&models.EquityTrade{Symbol: "AAPL", Tape: math.MaxInt8 + 1})
				return err
			},
			"trade condition": func() error {
				_, err := tradeFromMassive(&models.EquityTrade{Symbol: "AAPL", Conditions: []int32{1, math.MinInt16 - 1}})
				return err
			},
			"quote bid exchange": func() error {
				_, err := quoteFromMassive(&models.EquityQuote{Symbol: "AAPL", BidExchangeID: math.MaxInt16 + 1})
				return err
			},
			"quote ask exchange": func() error {
				_, err := quoteFromMassive(&models.EquityQuote{Symbol: "AAPL", AskExchangeID: math.MaxInt16 + 1})
				return err
			},
			"quote condition": func() error {
				_, err := quoteFromMassive(&models.EquityQuote{Symbol: "AAPL", Condition: math.MaxInt16 + 1})
				return err
			},
			"quote tape": func() error {
				_, err := quoteFromMassive(&models.EquityQuote{Symbol: "AAPL", Tape: math.MaxInt8 + 1})
				return err
			},
			"quote indicator": func() error {
				_, err := quoteFromMassive(&models.EquityQuote{Symbol: "AAPL", Indicators: []int32{math.MaxInt16 + 1}})
				return err
			},
		}

		for name, mapFn := range cases {
			if err := mapFn(); err == nil {
				t.Errorf("%s out of range should fail", name)
			}
		}
	})

	t.Run("an unexpected frame is an input error and does not hide the next one", func(t *testing.T) {
		inputErrors := 0
		frames := []any{
			models.EquityAgg{Symbol: "AAPL"},
			"not a frame at all",
			models.EquityTrade{Symbol: "AAPL", Timestamp: 42, Price: 241.35, Size: 10},
		}

		var events []gotrader.MarketEvent
		for _, frame := range frames {
			event, err := eventFromMessage(frame)
			if err != nil {
				inputErrors++
				continue
			}
			events = append(events, event)
		}

		if inputErrors != 2 {
			t.Errorf("input errors = %d, want 2", inputErrors)
		}

		if len(events) != 1 || !events[0].IsTrade() || events[0].TS() != 42 {
			t.Errorf("the valid trade was not produced: %+v", events)
		}
	})
}
