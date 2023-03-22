package alpacabroker

import (
	_ "github.com/joho/godotenv/autoload"
	"github.com/totomz/gotrader/gotrader"
	"log"
	"os"
	"testing"
	"time"
)

var (
	stdout    = log.New(os.Stdout, "", log.Lshortfile|log.Ltime)
	stderr    = log.New(os.Stdout, "[ERROR]", log.Lshortfile|log.Ltime|log.Lmsgprefix)
	apiKey    = os.Getenv("ALPACA_KEY")
	apiSecret = os.Getenv("ALPACA_SECRET")
	baseUrl   = "https://paper-api.alpaca.markets"

	c = gotrader.Candle{}

	alpa = NewAlpacaBroker(AlpacaBroker{
		Stdout: stdout,
		Stderr: stderr,
		// Signals: &gotrader.MemorySignals{
		// 	Metrics: map[string]*gotrader.TimeSerie{},
		// },
	}, apiKey, apiSecret, baseUrl)
)

func TestNewBroker(t *testing.T) {
	t.Skip("Manual test")
	cash := alpa.AvailableCash()
	if cash < 80000 {
		t.Errorf("You don't have money neither in the paper account?")
	}
}

func TestEmptyPosition(t *testing.T) {
	t.Skip("Manual test")
	position := alpa.GetPosition("AMZN")

	if position.Size != 0 {
		t.Fatal("expected 0")
	}
}

func TestOrderManagement(t *testing.T) {
	t.Skip("Manual test")
	orderId, err := alpa.SubmitOrder(c, gotrader.Order{
		Size:   1,
		Symbol: "VENAR",
		Type:   gotrader.OrderBuy,
	})
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(2 * time.Second)

	submitted, err := alpa.GetOrderByID(orderId)
	if err != nil {
		t.Fatal(err)
	}

	if submitted.Status != gotrader.OrderStatusFullFilled {
		t.Errorf("order has not been fullfilled")
	}

	if submitted.SizeFilled != 1 {
		t.Errorf("invalid filled size")
	}

	if submitted.Type != gotrader.OrderBuy {
		t.Errorf("supposed a buy")
	}

	position := alpa.GetPosition("VENAR")

	if position.Size != 1 {
		t.Errorf("invalid position size")
	}

	if position.AvgPrice > 1 {
		t.Errorf("VENAR above $1? WAT?")
	}

	err = alpa.ClosePosition(position)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(250 * time.Millisecond)

	positionClosed := alpa.GetPosition("VENAR")

	if positionClosed.Size != 0 {
		t.Errorf("invalid closed position size")
	}

	orderIdShort, err := alpa.SubmitOrder(c, gotrader.Order{
		Size:   1,
		Symbol: "TSLA",
		Type:   gotrader.OrderSell,
	})
	if err != nil {
		t.Fatal(err)
	}

	shortOrder, err := alpa.GetOrderByID(orderIdShort)
	if err != nil {
		t.Fatal(err)
	}

	if shortOrder.Type != gotrader.OrderSell {
		t.Fatal("expected a sell")
	}

	position = alpa.GetPosition("TSLA")
	err = alpa.ClosePosition(position)
	if err != nil {
		t.Fatal(err)
	}

}

func TestInvertPosition(t *testing.T) {

	t.Skip("TODO Manual test")
	_, err := alpa.SubmitOrder(c, gotrader.Order{
		Size:   3,
		Symbol: "TSLA",
		Type:   gotrader.OrderSell,
	})
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(1 * time.Second)

	p := alpa.GetPosition("TSLA")
	eClose := alpa.ClosePosition(p)
	if eClose != nil {
		t.Fatal(eClose)
	}

	_, err = alpa.SubmitOrder(c, gotrader.Order{
		Size:   3,
		Symbol: "TSLA",
		Type:   gotrader.OrderBuy,
	})
	if err != nil {
		t.Fatal(err)
	}

	pos := alpa.GetPosition("TSLA")
	println(pos.Size)
	println("e mo?")
}
