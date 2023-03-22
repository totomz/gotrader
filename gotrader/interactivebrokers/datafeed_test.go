package interactivebrokers

import (
	"github.com/hadrianl/ibapi"
	"github.com/totomz/gotrader/gotrader"
	"log"
	"os"
	"testing"
	"time"
)

func TestIbDataFeedGetCandles5Secs(t *testing.T) {

	stdout := log.New(os.Stdout, "", log.Lshortfile)
	stderr := log.New(os.Stdout, "[ERROR]", log.Lshortfile)

	ibClient, err := NewIbClientConnector(gateway, port, clientID, stdout, stderr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ibClient.Close()
	})

	dataFeed := DataFeed{
		Contracts: []*ibapi.Contract{&TSLA, &AMZN},
		IbClient:  &ibClient,
		Stdout:    stdout,
		Stderr:    stderr,
	}

	dataChan, err := dataFeed.Run()
	if err != nil {
		t.Error(err)
	}

	var amznCandles []gotrader.Candle
	var tslaCandles []gotrader.Candle

	go func() {
		for candle := range dataChan {
			switch candle.Symbol {
			case "AMZN":
				amznCandles = append(amznCandles, candle)
			case "TSLA":
				tslaCandles = append(tslaCandles, candle)
			default:
				t.Errorf("expdecting AMZN/TSLA got %v", candle.Symbol)
			}
		}
	}()

	time.Sleep(16 * time.Second)
	ibClient.Close()

	if len(amznCandles) != len(tslaCandles) || len(tslaCandles) != 3 {
		t.Errorf("Expecte 3 bars, got %v TSLA and %v AMZN", len(tslaCandles), len(amznCandles))
	}

}
