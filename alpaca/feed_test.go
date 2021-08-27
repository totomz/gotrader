package alpaca

import (
	"fmt"
	"github.com/alpacahq/alpaca-trade-api-go/alpaca"
	"github.com/alpacahq/alpaca-trade-api-go/common"
	"github.com/alpacahq/alpaca-trade-api-go/v2/stream"
	"os"
	"testing"
	"time"
)

func TestZio(t *testing.T) {
	t.Skip("alpaca is not supported")

	t.Cleanup(func() {
		stream.UnsubscribeQuotes("AAPL")
		stream.UnsubscribeBars("AAPL")
	})

	_ = os.Setenv(common.EnvApiKeyID, "PKKLJAA1U28RZ032Y09T")
	_ = os.Setenv(common.EnvApiSecretKey, "5APVkHoy2NcDI8eGwfge8sSlp8vp1iqwb0MRRVyR")

	fmt.Printf("Running w/ credentials [%v %v]\n", common.Credentials().ID, common.Credentials().Secret)

	alpaca.SetBaseUrl("https://paper-api.alpaca.markets")

	//alpacaClient := alpaca.NewClient(common.Credentials())
	//acct, err := alpacaClient.GetAccount()
	//if err != nil {
	//    panic(err)
	//}
	//fmt.Println(*acct)

	println(fmt.Sprintf("%v", time.Now()))
	if err := stream.SubscribeTrades(tradeHandler); err != nil {
		panic(err)
	}

	//if err := stream.SubscribeBars(barHandler, "AAPL"); err != nil {
	//	panic(err)
	//}
	//
	//if err := stream.SubscribeQuotes(quotehandler, "AAPL"); err != nil {
	//	panic(err)
	//}

	//if err := stream.SubscribeTradeUpdates(tradeUpdateshandler, "AAPL"); err != nil {
	//	panic(err)
	//}

	select {}
}

//func tradeUpdateshandler(u alpaca.TradeUpdate) {
//	println(fmt.Sprintf("", u.Event, u.Order.Symbol, u.Order.FilledQty))
//}

func tradeHandler(trademsg stream.Trade) {
	println(fmt.Sprintf("trade: %v:%v -- %v %v @ %v", trademsg.Tape, trademsg.ID, trademsg.Exchange, trademsg.Symbol, trademsg.Price))
}

func quotehandler(q stream.Quote) {
	println(fmt.Sprintf("[%s] bid:$%v [%v]....ask:$%v [%v]", q.Timestamp, q.BidPrice, q.BidSize, q.AskPrice, q.AskSize))
}

func barHandler(bar stream.Bar) {
	fmt.Println("bar", bar)
}
