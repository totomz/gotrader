package interactivebrokers

import (
	"fmt"
	"github.com/hadrianl/ibapi"
	"github.com/pkg/errors"
	"github.com/totomz/gotrader/gotrader"
	"go.uber.org/zap"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

var (
	AMZN = ibapi.Contract{
		ContractID:   3691937,
		Symbol:       "AMZN",
		SecurityType: "STK",
		Currency:     "USD",
		Exchange:     "SMART",
	}

	TSLA = ibapi.Contract{
		ContractID:   76792991,
		Symbol:       "TSLA",
		SecurityType: "STK",
		Currency:     "USD",
		Exchange:     "SMART",
	}
)

type AccountUpdate struct {
	Account  string
	Tag      string
	Value    string
	Currency string
}

type IbClientConnector struct {
	api       *ibapi.IbClient
	apiChan   *sync.Map
	apiErrors *sync.Map
	orderMux  sync.Mutex
	wrapper   *WrapperChannel
}

func NewIbClientConnector(gatewayHost string, gatewayPort int, clientID int64) (*IbClientConnector, error) {

	sharedResponseChannels := sync.Map{}
	sharedErrorsChannel := sync.Map{}

	wrapper := WrapperChannel{
		responseData:   &sharedResponseChannels,
		responseErrors: &sharedErrorsChannel,
		orderCache:     map[int64]*gotrader.Order{},
	}

	ic := ibapi.NewIbClient(&wrapper)

	if err := ic.Connect(gatewayHost, gatewayPort, clientID); err != nil {
		return &IbClientConnector{}, errors.Wrap(err, "error connecting to the API Gateway")
	}

	if err := ic.HandShake(); err != nil {
		return &IbClientConnector{}, errors.Wrap(err, "handShake failed")
	}
	err := ic.Run()
	if err != nil {
		return &IbClientConnector{}, err
	}

	/*
		(Writing to an unbuffered channel is blocking, until someone get the message.)
		ic.Disconnect() send a boolean in to a done channel, unbuffered, blocking the thread if there are
		no receiver.
		LoopUntilDone routine is the only channel receiver. Not sure why, but I need to manually start this routine...
	*/
	go func() {
		_ = ic.LoopUntilDone(func() {
			// do nothing
			// println(".")
		})
	}()

	connector := &IbClientConnector{
		api:       ic,
		wrapper:   &wrapper,
		apiChan:   &sharedResponseChannels,
		apiErrors: &sharedErrorsChannel,
	}

	return connector, nil
}

func (ib *IbClientConnector) Close() {
	err := ib.api.Disconnect()
	if err != nil {
		gotrader.Stderr.Println(err)
	}
}

func (ib *IbClientConnector) SubscribeMarketData5sBar(contract *ibapi.Contract) (<-chan ibapi.RealTimeBar, <-chan error) {

	respData, respErrors := ib.wrapApiChannels(func(reqID int64) {
		ib.api.ReqRealTimeBars(reqID, contract, 5, "MIDPOINT", false, nil)
	})

	barData := make(chan ibapi.RealTimeBar)

	go func() {
		for a := range respData {
			barData <- a.(ibapi.RealTimeBar)
		}

		for b := range respErrors {
			gotrader.Stdout.Printf("error: %v", b)
		}
	}()

	return barData, respErrors
}

func (ib *IbClientConnector) PlaceOrder(action string, qty int64, contract ibapi.Contract) (string, error) {
	ib.orderMux.Lock()
	defer ib.orderMux.Unlock()

	// Get the next order id.
	// God, this is sooo shitty
	ib.api.ReqIDs()
	orderId := ib.wrapper.orderID

	order := ibapi.NewMarketOrder(action, float64(qty))
	ib.api.PlaceOrder(orderId, &contract, order)

	return fmt.Sprintf("%v", orderId), nil
}

func (ib *IbClientConnector) AvailableFunds(accountName string) (float64, error) {
	res, err := ib.scalarResponse(ib.wrapApiChannels(func(reqID int64) {
		ib.api.ReqAccountSummary(reqID, accountName, "AvailableFunds")
	}))

	val, err := strconv.ParseFloat(res.(AccountUpdate).Value, 64)
	return val, err
}

func (ib *IbClientConnector) scalarResponse(respData <-chan interface{}, respErrors <-chan error) (interface{}, error) {
	var daje interface{}
	var err error
	for a := range respData {
		daje = a
	}

	for b := range respErrors {
		err = b
	}

	return daje, err
}

func (ib *IbClientConnector) wrapApiChannels(f func(reqID int64)) (<-chan interface{}, <-chan error) {
	reqID := ib.api.GetReqID()
	respData := make(chan interface{})
	respErrors := make(chan error)

	ib.apiChan.Store(reqID, respData)
	ib.apiErrors.Store(reqID, respErrors)

	f(reqID)

	return respData, respErrors

}

func (ib *IbClientConnector) ReqContractDetails(contract ibapi.Contract) ([]*ibapi.ContractDetails, error) {

	respData, respErrors := ib.wrapApiChannels(func(reqID int64) {
		ib.api.ReqContractDetails(reqID, &contract)
	})

	var res []*ibapi.ContractDetails
	for contractDetails := range respData {
		res = append(res, contractDetails.(*ibapi.ContractDetails))
	}

	// If there is an error, the wrapper **must** close the response channel!
	var err error
	for e := range respErrors {
		if err == nil {
			err = e
		} else {
			err = errors.Wrap(err, e.Error())
		}
	}

	return res, err
}

func ibOrderStateMap(orderState string) gotrader.OrderStatus {
	var orderStatus gotrader.OrderStatus
	switch orderState {
	case "ApiPending":
		orderStatus = gotrader.OrderStatusSubmitted
	case "PendingSubmit":
		orderStatus = gotrader.OrderStatusSubmitted
	case "PendingCancel":
		orderStatus = gotrader.OrderStatusSubmitted
	case "PreSubmitted":
		orderStatus = gotrader.OrderStatusSubmitted
	case "Submitted":
		orderStatus = gotrader.OrderStatusAccepted
	case "ApiCancelled":
		orderStatus = gotrader.OrderStatusRejected
	case "Cancelled":
		orderStatus = gotrader.OrderStatusRejected
	case "Filled":
		orderStatus = gotrader.OrderStatusFullFilled
	case "Inactive":
		orderStatus = gotrader.OrderStatusSubmitted
	default:
		panic(fmt.Sprintf("unsupported error status %v", orderState))

	}
	return orderStatus
}

// WrapperChannel is the default wrapper provided by this golang implement.
type WrapperChannel struct {
	orderID        int64
	orderCache     map[int64]*gotrader.Order
	responseData   *sync.Map
	responseErrors *sync.Map
}

func (w *WrapperChannel) GetNextOrderID() (i int64) {
	i = w.orderID
	atomic.AddInt64(&w.orderID, 1)
	return
}

func (w *WrapperChannel) ConnectAck() {
	gotrader.Stdout.Println("<ConnectAck>...")
}

func (w *WrapperChannel) ConnectionClosed() {
	gotrader.Stdout.Println("<ConnectionClosed>...")
}

func (w *WrapperChannel) NextValidID(reqID int64) {
	atomic.StoreInt64(&w.orderID, reqID)
}

func (w *WrapperChannel) ManagedAccounts(accountsList []string) {
	gotrader.Stdout.Println("<ManagedAccounts>", zap.Strings("accountList", accountsList))
}

func (w *WrapperChannel) TickPrice(_ int64, _ int64, _ float64, _ ibapi.TickAttrib) {
	// func (w *WrapperChannel) TickPrice(reqID int64, tickType int64, price float64, attrib ibapi.TickAttrib) {
}

func (w *WrapperChannel) UpdateAccountTime(accTime time.Time) {
	gotrader.Stdout.Println("<UpdateAccountTime>", zap.Time("accountTime", accTime))
}

func (w *WrapperChannel) UpdateAccountValue(tag string, value string, currency string, account string) {
	gotrader.Stdout.Println("<UpdateAccountValue>", zap.String("tag", tag), zap.String("value", value), zap.String("currency", currency), zap.String("account", account))
}

func (w *WrapperChannel) AccountDownloadEnd(accName string) {
	gotrader.Stdout.Println("<AccountDownloadEnd>", zap.String("accountName", accName))
}

func (w *WrapperChannel) AccountUpdateMulti(_ int64, _ string, _ string, _ string, _ string, _ string) {
	// func (w *WrapperChannel) AccountUpdateMulti(reqID int64, account string, modelCode string, tag string, value string, currency string) {
	gotrader.Stdout.Println("<AccountUpdateMulti>")
}

func (w *WrapperChannel) AccountUpdateMultiEnd(_ int64) {
	// func (w *WrapperChannel) AccountUpdateMultiEnd(reqID int64) {
	gotrader.Stdout.Println("<AccountUpdateMultiEnd>")
}

func (w *WrapperChannel) AccountSummary(reqID int64, account string, tag string, value string, currency string) {
	// gotrader.Stdout.Println("<AccountSummary>")
	channel, hasChannel := w.responseData.Load(reqID)
	if !hasChannel {
		gotrader.Stdout.Println("[WARNING] got a <AccountSummary> message, but there is no channel...")
	}
	channel.(chan interface{}) <- AccountUpdate{
		Account:  account,
		Tag:      tag,
		Value:    value,
		Currency: currency,
	}
}

func (w *WrapperChannel) AccountSummaryEnd(reqID int64) {
	// gotrader.Stdout.Println("AccountSummaryEnd ma non è del tutto vero mi sa")
	closeChannels(w, reqID)
}

func (w *WrapperChannel) VerifyMessageAPI(apiData string) {
	gotrader.Stdout.Println("<VerifyMessageAPI>", zap.String("apiData", apiData))
}

func (w *WrapperChannel) VerifyCompleted(isSuccessful bool, err string) {
	gotrader.Stdout.Println("<VerifyCompleted>", zap.Bool("isSuccessful", isSuccessful), zap.String("error", err))
}

func (w *WrapperChannel) VerifyAndAuthMessageAPI(apiData string, xyzChallange string) {
	gotrader.Stdout.Println("<VerifyMessageAPI>", zap.String("apiData", apiData), zap.String("xyzChallange", xyzChallange))
}

func (w *WrapperChannel) VerifyAndAuthCompleted(isSuccessful bool, err string) {
	gotrader.Stdout.Println("<VerifyCompleted>", zap.Bool("isSuccessful", isSuccessful), zap.String("error", err))
}

func (w *WrapperChannel) DisplayGroupList(_ int64, _ string) {
	// func (w *WrapperChannel) DisplayGroupList(reqID int64, groups string) {
	gotrader.Stdout.Println("<DisplayGroupList>")
}

func (w *WrapperChannel) DisplayGroupUpdated(_ int64, _ string) {
	// func (w *WrapperChannel) DisplayGroupUpdated(reqID int64, contractInfo string) {
	gotrader.Stdout.Println("<DisplayGroupUpdated>")
}

func (w *WrapperChannel) PositionMulti(_ int64, _ string, _ string, _ *ibapi.Contract, _ float64, _ float64) {
	// func (w *WrapperChannel) PositionMulti(reqID int64, account string, modelCode string, contract *ibapi.Contract, position float64, avgCost float64) {
	gotrader.Stdout.Println("<PositionMulti>")
}

func (w *WrapperChannel) PositionMultiEnd(_ int64) {
	// func (w *WrapperChannel) PositionMultiEnd(reqID int64) {
	gotrader.Stdout.Println("<PositionMultiEnd>")
}

func (w *WrapperChannel) UpdatePortfolio(_ *ibapi.Contract, _ float64, _ float64, _ float64, _ float64, _ float64, _ float64, _ string) {
	// func (w *WrapperChannel) UpdatePortfolio(contract *ibapi.Contract, position float64, marketPrice float64, marketValue float64, averageCost float64, unrealizedPNL float64, realizedPNL float64, accName string) {
	gotrader.Stdout.Println("<UpdatePortfolio>")
}

func (w *WrapperChannel) Position(_ string, _ *ibapi.Contract, _ float64, _ float64) {
	// func (w *WrapperChannel) Position(account string, contract *ibapi.Contract, position float64, avgCost float64) {
	gotrader.Stdout.Println("<UpdatePortfolio>")
}

func (w *WrapperChannel) PositionEnd() {
	gotrader.Stdout.Println("<PositionEnd>")
}

func (w *WrapperChannel) Pnl(_ int64, _ float64, _ float64, _ float64) {
	// func (w *WrapperChannel) Pnl(reqID int64, dailyPnL float64, unrealizedPnL float64, realizedPnL float64) {
	gotrader.Stdout.Println("<PNL>")
}

func (w *WrapperChannel) PnlSingle(_ int64, _ int64, _ float64, _ float64, _ float64, _ float64) {
	// func (w *WrapperChannel) PnlSingle(reqID int64, position int64, dailyPnL float64, unrealizedPnL float64, realizedPnL float64, value float64) {
	gotrader.Stdout.Println("<PNLSingle>")
}

func (w *WrapperChannel) OpenOrder(orderID int64, contract *ibapi.Contract, order *ibapi.Order, orderState *ibapi.OrderState) {

	var otype gotrader.OrderType
	switch order.Action {
	case "BUY":
		otype = gotrader.OrderBuy
	case "SELL":
		otype = gotrader.OrderSell
	default:
		panic(fmt.Sprintf("unkown order type %v", order.Action))
	}

	orderStatus := ibOrderStateMap(orderState.Status)

	openorder := gotrader.Order{
		Id:         fmt.Sprintf("%v", orderID),
		Size:       int64(order.TotalQuantity),
		Symbol:     gotrader.Symbol(contract.Symbol),
		Type:       otype,
		Status:     orderStatus,
		SizeFilled: int64(order.FilledQuantity),
	}

	w.orderCache[orderID] = &openorder
	gotrader.Stdout.Println("<OpenOrder>")
}

func (w *WrapperChannel) OpenOrderEnd() {
	gotrader.Stdout.Println("<OpenOrderEnd>")

}

func (w *WrapperChannel) OrderStatus(orderID int64, status string, filled float64, _ float64, _ float64, _ int64, _ int64, _ float64, _ int64, _ string, _ float64) {
	// func (w *WrapperChannel) OrderStatus(orderID int64, status string, filled float64, remaining float64, avgFillPrice float64, permID int64, parentID int64, lastFillPrice float64, clientID int64, whyHeld string, mktCapPrice float64) {
	order, found := w.orderCache[orderID]
	if !found {
		gotrader.Stdout.Println(fmt.Sprintf("gor update for an unkonwn order :( orderId:%v status:%v", orderID, status))
	}

	orderStatus := ibOrderStateMap(status)
	order.Status = orderStatus
	order.SizeFilled = int64(filled)

	gotrader.Stdout.Println("<OrderStatus>")
}

func (w *WrapperChannel) ExecDetails(_ int64, _ *ibapi.Contract, _ *ibapi.Execution) {
	// func (w *WrapperChannel) ExecDetails(reqID int64, contract *ibapi.Contract, execution *ibapi.Execution) {
	gotrader.Stdout.Println("<ExecDetails>")
}

func (w *WrapperChannel) ExecDetailsEnd(_ int64) {
	// func (w *WrapperChannel) ExecDetailsEnd(reqID int64) {
	gotrader.Stdout.Println("<ExecDetailsEnd>")
}

func (w *WrapperChannel) DeltaNeutralValidation(_ int64, _ ibapi.DeltaNeutralContract) {
	// func (w *WrapperChannel) DeltaNeutralValidation(reqID int64, deltaNeutralContract ibapi.DeltaNeutralContract) {
	gotrader.Stdout.Println("<DeltaNeutralValidation>")
}

func (w *WrapperChannel) CommissionReport(_ ibapi.CommissionReport) {
	// func (w *WrapperChannel) CommissionReport(commissionReport ibapi.CommissionReport) {
	gotrader.Stdout.Println("<CommissionReport>")
}

func (w *WrapperChannel) OrderBound(_ int64, _ int64, _ int64) {
	// func (w *WrapperChannel) OrderBound(reqID int64, apiClientID int64, apiOrderID int64) {
	gotrader.Stdout.Println("<OrderBound>")
}

func (w *WrapperChannel) ContractDetails(reqID int64, conDetails *ibapi.ContractDetails) {
	// gotrader.Stdout.Println("<ContractDetails>")
	channel, _ := w.responseData.Load(reqID)
	channel.(chan interface{}) <- conDetails
}

func (w *WrapperChannel) ContractDetailsEnd(reqID int64) {
	// gotrader.Stdout.Println("<ContractDetailsEnd>")
	closeChannels(w, reqID)
}

func closeChannels(w *WrapperChannel, reqID int64) {
	data, hasData := w.responseData.Load(reqID)
	errs, hasErrors := w.responseErrors.Load(reqID)

	if hasData {
		close(data.(chan interface{}))
	}

	if hasErrors {
		close(errs.(chan error))
	}

}

func (w *WrapperChannel) BondContractDetails(_ int64, _ *ibapi.ContractDetails) {
	// func (w *WrapperChannel) BondContractDetails(reqID int64, conDetails *ibapi.ContractDetails) {
	gotrader.Stdout.Println("<BondContractDetails>")
}

func (w *WrapperChannel) SymbolSamples(_ int64, _ []ibapi.ContractDescription) {
	// func (w *WrapperChannel) SymbolSamples(reqID int64, contractDescriptions []ibapi.ContractDescription) {
	gotrader.Stdout.Println("<SymbolSamples>")
}

func (w *WrapperChannel) SmartComponents(_ int64, _ []ibapi.SmartComponent) {
	// func (w *WrapperChannel) SmartComponents(reqID int64, smartComps []ibapi.SmartComponent) {
	gotrader.Stdout.Println("<SmartComponents>")
}

func (w *WrapperChannel) MarketRule(_ int64, _ []ibapi.PriceIncrement) {
	// func (w *WrapperChannel) MarketRule(marketRuleID int64, priceIncrements []ibapi.PriceIncrement) {
	gotrader.Stdout.Println("<MarketRule>")
}

func (w *WrapperChannel) RealtimeBar(reqID int64, t int64, open float64, high float64, low float64, close float64, volume int64, wap float64, count int64) {
	// gotrader.Stdout.Printf("<RealtimeBar> %v %v ", time.Unix(t, 0).String(), low)
	channel, _ := w.responseData.Load(reqID)
	bar := ibapi.RealTimeBar{
		Time:   t,
		Open:   open,
		High:   high,
		Low:    low,
		Close:  close,
		Volume: volume,
		Wap:    wap,
		Count:  count,
	}
	channel.(chan interface{}) <- bar
}

func (w *WrapperChannel) HistoricalData(_ int64, _ *ibapi.BarData) {
	// func (w *WrapperChannel) HistoricalData(reqID int64, bar *ibapi.BarData) {
	gotrader.Stdout.Println("<HistoricalData>")
}

func (w *WrapperChannel) HistoricalDataEnd(_ int64, _ string, _ string) {
	// func (w *WrapperChannel) HistoricalDataEnd(reqID int64, startDateStr string, endDateStr string) {
	gotrader.Stdout.Println("<HistoricalDataEnd>")
}

func (w *WrapperChannel) HistoricalDataUpdate(_ int64, _ *ibapi.BarData) {
	// func (w *WrapperChannel) HistoricalDataUpdate(reqID int64, bar *ibapi.BarData) {
	gotrader.Stdout.Println("<HistoricalDataUpdate>")
}

func (w *WrapperChannel) HeadTimestamp(_ int64, _ string) {
	// func (w *WrapperChannel) HeadTimestamp(reqID int64, headTimestamp string) {
	gotrader.Stdout.Println("<HeadTimestamp>")
}

func (w *WrapperChannel) HistoricalTicks(_ int64, _ []ibapi.HistoricalTick, _ bool) {
	// func (w *WrapperChannel) HistoricalTicks(reqID int64, ticks []ibapi.HistoricalTick, done bool) {
	gotrader.Stdout.Println("<HistoricalTicks>")
}

func (w *WrapperChannel) HistoricalTicksBidAsk(_ int64, _ []ibapi.HistoricalTickBidAsk, _ bool) {
	// func (w *WrapperChannel) HistoricalTicksBidAsk(reqID int64, ticks []ibapi.HistoricalTickBidAsk, done bool) {
	gotrader.Stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}

func (w *WrapperChannel) HistoricalTicksLast(_ int64, _ []ibapi.HistoricalTickLast, _ bool) {
	// func (w *WrapperChannel) HistoricalTicksLast(reqID int64, ticks []ibapi.HistoricalTickLast, done bool) {
	gotrader.Stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}

func (w *WrapperChannel) TickSize(_ int64, _ int64, _ int64) {
	// func (w *WrapperChannel) TickSize(reqID int64, tickType int64, size int64) {
	gotrader.Stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}

func (w *WrapperChannel) TickSnapshotEnd(_ int64) {
	// func (w *WrapperChannel) TickSnapshotEnd(reqID int64) {
	gotrader.Stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}

func (w *WrapperChannel) MarketDataType(_ int64, _ int64) {
	// func (w *WrapperChannel) MarketDataType(reqID int64, marketDataType int64) {
	gotrader.Stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}

func (w *WrapperChannel) TickByTickAllLast(_ int64, _ int64, _ int64, _ float64, _ int64, _ ibapi.TickAttribLast, _ string, _ string) {
	// func (w *WrapperChannel) TickByTickAllLast(reqID int64, tickType int64, time int64, price float64, size int64, tickAttribLast ibapi.TickAttribLast, exchange string, specialConditions string) {
	gotrader.Stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}

func (w *WrapperChannel) TickByTickBidAsk(_ int64, _ int64, _ float64, _ float64, _ int64, _ int64, _ ibapi.TickAttribBidAsk) {
	// func (w *WrapperChannel) TickByTickBidAsk(reqID int64, time int64, bidPrice float64, askPrice float64, bidSize int64, askSize int64, tickAttribBidAsk ibapi.TickAttribBidAsk) {
	gotrader.Stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}

func (w *WrapperChannel) TickByTickMidPoint(_ int64, _ int64, _ float64) {
	// func (w *WrapperChannel) TickByTickMidPoint(reqID int64, time int64, midPoint float64) {
	gotrader.Stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}

func (w *WrapperChannel) TickString(_ int64, _ int64, _ string) {
	// func (w *WrapperChannel) TickString(reqID int64, tickType int64, value string) {
	gotrader.Stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}

func (w *WrapperChannel) TickGeneric(_ int64, _ int64, _ float64) {
	// func (w *WrapperChannel) TickGeneric(reqID int64, tickType int64, value float64) {
	gotrader.Stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}

func (w *WrapperChannel) TickEFP(_ int64, _ int64, _ float64, _ string, _ float64, _ int64, _ string, _ float64, _ float64) {
	// func (w *WrapperChannel) TickEFP(reqID int64, tickType int64, basisPoints float64, formattedBasisPoints string, totalDividends float64, holdDays int64, futureLastTradeDate string, dividendImpact float64, dividendsToLastTradeDate float64) {
	gotrader.Stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}

func (w *WrapperChannel) TickReqParams(_ int64, _ float64, _ string, _ int64) {
	// func (w *WrapperChannel) TickReqParams(reqID int64, minTick float64, bboExchange string, snapshotPermissions int64) {
	gotrader.Stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}
func (w *WrapperChannel) MktDepthExchanges(_ []ibapi.DepthMktDataDescription) {
	// func (w *WrapperChannel) MktDepthExchanges(depthMktDataDescriptions []ibapi.DepthMktDataDescription) {
	gotrader.Stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}

/*
UpdateMktDepth
Returns the order book.
tickerId -  the request's identifier
position -  the order book's row being updated
operation - how to refresh the row:

	0 = insert (insert this new order into the row identified by 'position')
	1 = update (update the existing order in the row identified by 'position')
	2 = delete (delete the existing order at the row identified by 'position').

side -  0 for ask, 1 for bid
price - the order's price
size -  the order's size
*/
func (w *WrapperChannel) UpdateMktDepth(_ int64, _ int64, _ int64, _ int64, _ float64, _ int64) {
	// func (w *WrapperChannel) UpdateMktDepth(reqID int64, position int64, operation int64, side int64, price float64, size int64) {
	gotrader.Stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}

func (w *WrapperChannel) UpdateMktDepthL2(_ int64, _ int64, _ string, _ int64, _ int64, _ float64, _ int64, _ bool) {
	// func (w *WrapperChannel) UpdateMktDepthL2(reqID int64, position int64, marketMaker string, operation int64, side int64, price float64, size int64, isSmartDepth bool) {
	gotrader.Stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}

func (w *WrapperChannel) TickOptionComputation(_ int64, _ int64, _ float64, _ float64, _ float64, _ float64, _ float64, _ float64, _ float64, _ float64) {
	// func (w *WrapperChannel) TickOptionComputation(reqID int64, tickType int64, impliedVol float64, delta float64, optPrice float64, pvDiviedn float64, gamma float64, vega float64, theta float64, undPrice float64) {
	gotrader.Stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}

func (w *WrapperChannel) FundamentalData(_ int64, _ string) {
	// func (w *WrapperChannel) FundamentalData(reqID int64, data string) {
	gotrader.Stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}

func (w *WrapperChannel) ScannerParameters(_ string) {
	// func (w *WrapperChannel) ScannerParameters(xml string) {
	gotrader.Stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}

func (w *WrapperChannel) ScannerData(_ int64, _ int64, _ *ibapi.ContractDetails, _ string, _ string, _ string, _ string) {
	// func (w *WrapperChannel) ScannerData(reqID int64, rank int64, conDetails *ibapi.ContractDetails, distance string, benchmark string, projection string, legs string) {
	gotrader.Stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}

func (w *WrapperChannel) ScannerDataEnd(_ int64) {
	// func (w *WrapperChannel) ScannerDataEnd(reqID int64) {
	gotrader.Stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}

func (w *WrapperChannel) HistogramData(_ int64, _ []ibapi.HistogramData) {
	// func (w *WrapperChannel) HistogramData(reqID int64, histogram []ibapi.HistogramData) {
	gotrader.Stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}

func (w *WrapperChannel) RerouteMktDataReq(_ int64, _ int64, _ string) {
	// func (w *WrapperChannel) RerouteMktDataReq(reqID int64, contractID int64, exchange string) {
	gotrader.Stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}

func (w *WrapperChannel) RerouteMktDepthReq(_ int64, _ int64, _ string) {
	// func (w *WrapperChannel) RerouteMktDepthReq(reqID int64, contractID int64, exchange string) {
	gotrader.Stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}

func (w *WrapperChannel) SecurityDefinitionOptionParameter(_ int64, _ string, _ int64, _ string, _ string, _ []string, _ []float64) {
	// func (w *WrapperChannel) SecurityDefinitionOptionParameter(reqID int64, exchange string, underlyingContractID int64, tradingClass string, multiplier string, expirations []string, strikes []float64) {
	gotrader.Stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}

func (w *WrapperChannel) SecurityDefinitionOptionParameterEnd(_ int64) {
	// func (w *WrapperChannel) SecurityDefinitionOptionParameterEnd(reqID int64) {
	gotrader.Stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}

func (w *WrapperChannel) SoftDollarTiers(_ int64, _ []ibapi.SoftDollarTier) {
	// func (w *WrapperChannel) SoftDollarTiers(reqID int64, tiers []ibapi.SoftDollarTier) {
	gotrader.Stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}

func (w *WrapperChannel) FamilyCodes(_ []ibapi.FamilyCode) {
	// func (w *WrapperChannel) FamilyCodes(famCodes []ibapi.FamilyCode) {
	gotrader.Stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}

func (w *WrapperChannel) NewsProviders(_ []ibapi.NewsProvider) {
	// func (w *WrapperChannel) NewsProviders(newsProviders []ibapi.NewsProvider) {
	gotrader.Stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}

func (w *WrapperChannel) TickNews(_ int64, _ int64, _ string, _ string, _ string, _ string) {
	// func (w *WrapperChannel) TickNews(tickerID int64, timeStamp int64, providerCode string, articleID string, headline string, extraData string) {
	gotrader.Stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}

func (w *WrapperChannel) NewsArticle(_ int64, _ int64, _ string) {
	// func (w *WrapperChannel) NewsArticle(reqID int64, articleType int64, articleText string) {
	gotrader.Stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}

func (w *WrapperChannel) HistoricalNews(_ int64, _ string, _ string, _ string, _ string) {
	// func (w *WrapperChannel) HistoricalNews(reqID int64, time string, providerCode string, articleID string, headline string) {
	gotrader.Stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}

func (w *WrapperChannel) HistoricalNewsEnd(_ int64, _ bool) {
	// func (w *WrapperChannel) HistoricalNewsEnd(reqID int64, hasMore bool) {
	gotrader.Stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}

func (w *WrapperChannel) UpdateNewsBulletin(_ int64, _ int64, _ string, _ string) {
	// func (w *WrapperChannel) UpdateNewsBulletin(msgID int64, msgType int64, newsMessage string, originExch string) {
	gotrader.Stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}

func (w *WrapperChannel) ReceiveFA(_ int64, _ string) {
	// func (w *WrapperChannel) ReceiveFA(faData int64, cxml string) {
	gotrader.Stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}

func (w *WrapperChannel) CurrentTime(_ time.Time) {
	// func (w *WrapperChannel) CurrentTime(t time.Time) {
	gotrader.Stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}

func (w *WrapperChannel) Error(reqID int64, errCode int64, errString string) {
	data, hasDataChannel := w.responseData.Load(reqID)
	if hasDataChannel {
		close(data.(chan interface{}))
	}

	err, hasErrChannel := w.responseErrors.Load(reqID)
	if hasErrChannel {
		ch := err.(chan error)
		ch <- errors.Errorf("[ibapi] (%v) ERROR %v - %s", reqID, errCode, errString)
		close(ch)
	}

	gotrader.Stdout.Printf("[ibapi] (%v) ERROR %v - %s", reqID, errCode, errString)
}

func (w *WrapperChannel) CompletedOrder(_ *ibapi.Contract, _ *ibapi.Order, _ *ibapi.OrderState) {
	// func (w *WrapperChannel) CompletedOrder(contract *ibapi.Contract, order *ibapi.Order, orderState *ibapi.OrderState) {
	gotrader.Stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}

func (w *WrapperChannel) CompletedOrdersEnd() {
	gotrader.Stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}

func (w *WrapperChannel) ReplaceFAEnd(_ int64, _ string) {
	// func (w *WrapperChannel) ReplaceFAEnd(reqID int64, text string) {
	gotrader.Stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}
