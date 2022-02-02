package alpacabroker

import (
	_ "github.com/joho/godotenv/autoload"
	"github.com/totomz/gotrader"
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

	alpa = NewAlpacaBroker(AlpacaBroker{
		Stdout: stdout,
		Stderr: stderr,
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
	position, err := alpa.GetPosition("AMZN")
	if err != nil {
		t.Fatal(err)
	}

	if position.Size != 0 {
		t.Fatal("expected 0")
	}
}

func TestOrderManagement(t *testing.T) {
	t.Skip("Manual test")
	orderId, err := alpa.SubmitOrder(gotrader.Order{
		Size:   1,
		Symbol: "VENAR",
		Type:   gotrader.OrderBuy,
	})
	if err != nil {
		t.Fatal(err)
	}

	println(orderId)

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

	position, err := alpa.GetPosition("VENAR")
	if err != nil {
		t.Fatal(err)
	}

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

	positionClosed, err := alpa.GetPosition("VENAR")
	if err != nil {
		t.Fatal(err)
	}

	if positionClosed.Size != 0 {
		t.Errorf("invalid closed position size")
	}

}