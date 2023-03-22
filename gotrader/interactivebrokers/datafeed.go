package interactivebrokers

import (
	"fmt"
	"github.com/hadrianl/ibapi"
	"github.com/totomz/gotrader/gotrader"
	"time"
)

type DataFeed struct {
	IbClient  *IbClientConnector
	Contracts []*ibapi.Contract
}

func (feed *DataFeed) Run() (chan gotrader.Candle, error) {

	feedCandles := make(chan gotrader.Candle, len(feed.Contracts))

	for i := range feed.Contracts {
		contract := feed.Contracts[i]
		gotrader.Stdout.Println(fmt.Sprintf("Starting feed %v", contract))
		dataChannel, errorChannel := feed.IbClient.SubscribeMarketData5sBar(contract)

		go func() {
			for bar := range dataChannel {
				gotrader.Stdout.Printf("Bar: %s", bar.String())

				// La ritorno al channel
				candle := gotrader.Candle{
					Open:   bar.Open,
					High:   bar.High,
					Close:  bar.Close,
					Low:    bar.Low,
					Volume: bar.Volume,
					Symbol: gotrader.Symbol(contract.Symbol),
					Time:   time.Unix(bar.Time, 0),
				}

				select {
				case feedCandles <- candle:
					// message sent
				default:
					gotrader.Stdout.Printf("bar dropped %v", bar)
				}
			}
		}()

		go func() {
			for err := range errorChannel {
				gotrader.Stderr.Printf("ERROR - %v", err)
			}
		}()

	}
	return feedCandles, nil
}

// func aazio() {
//
//
//	// implement your own IbWrapper to handle the msg delivered via tws or gateway
//    // Wrapper{} below is a default implement which just log the msg
//    ic := ibapi.NewIbClient(&ibapi.Wrapper{
//
//	})
//
//    if err := ic.Connect("127.0.0.1", 7497, 101);err != nil {
//        log.Panic("Connect failed:", err)
//    }
//
//    if err := ic.HandShake();err != nil {
//        log.Panic("HandShake failed:", err)
//    }
//
//    //ic.ReqCurrentTime()
//    //ic.ReqAutoOpenOrders(true)
//    //ic.ReqAccountUpdates(true, "")
//    //ic.ReqExecutions(ic.GetReqID(), ibapi.ExecutionFilter{})
//
//    // start to send req and receive msg from tws or gateway after this
//    err := ic.Run()
//    if err != nil {
//    	panic(err)
//	}
//	defer ic.Disconnect()
//
//    // ID 3691937
//	ic.ReqContractDetails(ic.GetReqID(), &ibapi.Contract{
//		Symbol: "AMZN",
//		SecurityType: "STK",
//		Currency: "USD",
//		Exchange: "SMART",
//	})
//
//    amzn := &ibapi.Contract{
//    	ContractID: 3691937,
//		Symbol: "AMZN",
//		SecurityType: "STK",
//		Currency: "USD",
//		Exchange: "SMART",
//	}
//
//	// bar 5 secondi, barSize ignorato
//    ic.ReqRealTimeBars(ic.GetReqID(), amzn, 5, "MIDPOINT", false, nil)
//
//
//    <-time.After(time.Second * 60)
//
// }
