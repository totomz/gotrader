package alpacabroker

import (
	"fmt"
	"github.com/alpacahq/alpaca-trade-api-go/v3/marketdata"
	_ "github.com/joho/godotenv/autoload"
	"github.com/totomz/gotrader"
	"os"
	"testing"
)

func TestDataFeedLive(t *testing.T) {
	t.Skip("Manual test")

	feed := &DataFeed{
		APIKey:    os.Getenv("ALPACA_KEY"),
		APISecret: os.Getenv("ALPACA_SECRET"),
		Symbols:   []gotrader.Symbol{"AAPL", "NFLX"},
		Feed:      marketdata.IEX,
	}

	candles, err := feed.Run()
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 10; i++ {
		candle, ok := <-candles
		if !ok {
			t.Fatal("stream closed before receiving 10 candles")
		}
		fmt.Printf("[%d] %s\n", i+1, candle.String())
	}
}
