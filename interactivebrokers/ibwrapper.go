package interactivebrokers

import (
	"fmt"
	"github.com/hadrianl/ibapi"
	"github.com/pkg/errors"
	"github.com/totomz/gotrader"
	"go.uber.org/zap"
	"log"
	"os"
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

	Stdout *log.Logger
	Stderr *log.Logger
}

func NewIbClientConnector(gatewayHost string, gatewayPort int, clientID int64, stdout, stderr *log.Logger) (IbClientConnector, error) {

	if stdout == nil {
		stdout = log.New(os.Stdout, "", log.Lshortfile|log.Ltime)
	}
	if stderr == nil {
		stderr = log.New(os.Stdout, "[ERROR]", log.Lshortfile|log.Ltime)
	}

	sharedResponseChannels := sync.Map{}
	sharedErrorsChannel := sync.Map{}

	wrapper := WrapperChannel{
		responseData:   &sharedResponseChannels,
		responseErrors: &sharedErrorsChannel,
		orderCache:     map[int64]*gotrader.Order{},
		stdout:         stdout,
	}

	ic := ibapi.NewIbClient(&wrapper)

	if err := ic.Connect(gatewayHost, gatewayPort, clientID); err != nil {
		return IbClientConnector{}, errors.Wrap(err, "error connecting to the API Gateway")
	}

	if err := ic.HandShake(); err != nil {
		return IbClientConnector{}, errors.Wrap(err, "handShake failed")
	}
	err := ic.Run()
	if err != nil {
		return IbClientConnector{}, err
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
			println(".")
		})
	}()

	connector := IbClientConnector{
		api:       ic,
		wrapper:   &wrapper,
		apiChan:   &sharedResponseChannels,
		apiErrors: &sharedErrorsChannel,
		Stdout:    stdout,
		Stderr:    stderr,
	}

	return connector, nil
}

func (ib *IbClientConnector) Close() {
	err := ib.api.Disconnect()
	ib.Stderr.Println(err)
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
			ib.Stdout.Printf("error: %v", b)
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
	stdout         *log.Logger
}

func (w *WrapperChannel) GetNextOrderID() (i int64) {
	i = w.orderID
	atomic.AddInt64(&w.orderID, 1)
	return
}

func (w *WrapperChannel) ConnectAck() {
	w.stdout.Println("<ConnectAck>...")
}

func (w *WrapperChannel) ConnectionClosed() {
	w.stdout.Println("<ConnectionClosed>...")
}

func (w *WrapperChannel) NextValidID(reqID int64) {
	atomic.StoreInt64(&w.orderID, reqID)
}

func (w *WrapperChannel) ManagedAccounts(accountsList []string) {
	w.stdout.Println("<ManagedAccounts>", zap.Strings("accountList", accountsList))
}

func (w *WrapperChannel) TickPrice(reqID int64, tickType int64, price float64, attrib ibapi.TickAttrib) {
}

func (w *WrapperChannel) UpdateAccountTime(accTime time.Time) {
	w.stdout.Println("<UpdateAccountTime>", zap.Time("accountTime", accTime))
}

func (w *WrapperChannel) UpdateAccountValue(tag string, value string, currency string, account string) {
	w.stdout.Println("<UpdateAccountValue>", zap.String("tag", tag), zap.String("value", value), zap.String("currency", currency), zap.String("account", account))
}

func (w *WrapperChannel) AccountDownloadEnd(accName string) {
	w.stdout.Println("<AccountDownloadEnd>", zap.String("accountName", accName))
}

func (w *WrapperChannel) AccountUpdateMulti(reqID int64, account string, modelCode string, tag string, value string, currency string) {
	w.stdout.Println("<AccountUpdateMulti>")
}

func (w *WrapperChannel) AccountUpdateMultiEnd(reqID int64) {
	w.stdout.Println("<AccountUpdateMultiEnd>")
}

func (w *WrapperChannel) AccountSummary(reqID int64, account string, tag string, value string, currency string) {
	// w.stdout.Println("<AccountSummary>")
	channel, hasChannel := w.responseData.Load(reqID)
	if !hasChannel {
		w.stdout.Println("[WARNING] got a <AccountSummary> message, but there is no channel...")
	}
	channel.(chan interface{}) <- AccountUpdate{
		Account:  account,
		Tag:      tag,
		Value:    value,
		Currency: currency,
	}
}

func (w *WrapperChannel) AccountSummaryEnd(reqID int64) {
	// w.stdout.Println("AccountSummaryEnd ma non è del tutto vero mi sa")
	closeChannels(w, reqID)
}

func (w *WrapperChannel) VerifyMessageAPI(apiData string) {
	w.stdout.Println("<VerifyMessageAPI>", zap.String("apiData", apiData))
}

func (w *WrapperChannel) VerifyCompleted(isSuccessful bool, err string) {
	w.stdout.Println("<VerifyCompleted>", zap.Bool("isSuccessful", isSuccessful), zap.String("error", err))
}

func (w *WrapperChannel) VerifyAndAuthMessageAPI(apiData string, xyzChallange string) {
	w.stdout.Println("<VerifyMessageAPI>", zap.String("apiData", apiData), zap.String("xyzChallange", xyzChallange))
}

func (w *WrapperChannel) VerifyAndAuthCompleted(isSuccessful bool, err string) {
	w.stdout.Println("<VerifyCompleted>", zap.Bool("isSuccessful", isSuccessful), zap.String("error", err))
}

func (w *WrapperChannel) DisplayGroupList(reqID int64, groups string) {
	w.stdout.Println("<DisplayGroupList>")
}

func (w *WrapperChannel) DisplayGroupUpdated(reqID int64, contractInfo string) {
	w.stdout.Println("<DisplayGroupUpdated>")
}

func (w *WrapperChannel) PositionMulti(reqID int64, account string, modelCode string, contract *ibapi.Contract, position float64, avgCost float64) {
	w.stdout.Println("<PositionMulti>")
}

func (w *WrapperChannel) PositionMultiEnd(reqID int64) {
	w.stdout.Println("<PositionMultiEnd>")
}

func (w *WrapperChannel) UpdatePortfolio(contract *ibapi.Contract, position float64, marketPrice float64, marketValue float64, averageCost float64, unrealizedPNL float64, realizedPNL float64, accName string) {
	w.stdout.Println("<UpdatePortfolio>")
}

func (w *WrapperChannel) Position(account string, contract *ibapi.Contract, position float64, avgCost float64) {
	w.stdout.Println("<UpdatePortfolio>")
}

func (w *WrapperChannel) PositionEnd() {
	w.stdout.Println("<PositionEnd>")
}

func (w *WrapperChannel) Pnl(reqID int64, dailyPnL float64, unrealizedPnL float64, realizedPnL float64) {
	w.stdout.Println("<PNL>")
}

func (w *WrapperChannel) PnlSingle(reqID int64, position int64, dailyPnL float64, unrealizedPnL float64, realizedPnL float64, value float64) {
	w.stdout.Println("<PNLSingle>")
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
	w.stdout.Println("<OpenOrder>")
}

func (w *WrapperChannel) OpenOrderEnd() {
	w.stdout.Println("<OpenOrderEnd>")

}

func (w *WrapperChannel) OrderStatus(orderID int64, status string, filled float64, remaining float64, avgFillPrice float64, permID int64, parentID int64, lastFillPrice float64, clientID int64, whyHeld string, mktCapPrice float64) {
	order, found := w.orderCache[orderID]
	if !found {
		w.stdout.Println(fmt.Sprintf("gor update for an unkonwn order :( orderId:%v status:%v", orderID, status))
	}

	orderStatus := ibOrderStateMap(status)
	order.Status = orderStatus
	order.SizeFilled = int64(filled)

	w.stdout.Println("<OrderStatus>")
}

func (w *WrapperChannel) ExecDetails(reqID int64, contract *ibapi.Contract, execution *ibapi.Execution) {
	w.stdout.Println("<ExecDetails>")
}

func (w *WrapperChannel) ExecDetailsEnd(reqID int64) {
	w.stdout.Println("<ExecDetailsEnd>")
}

func (w *WrapperChannel) DeltaNeutralValidation(reqID int64, deltaNeutralContract ibapi.DeltaNeutralContract) {
	w.stdout.Println("<DeltaNeutralValidation>")
}

func (w *WrapperChannel) CommissionReport(commissionReport ibapi.CommissionReport) {
	w.stdout.Println("<CommissionReport>")
}

func (w *WrapperChannel) OrderBound(reqID int64, apiClientID int64, apiOrderID int64) {
	w.stdout.Println("<OrderBound>")
}

func (w *WrapperChannel) ContractDetails(reqID int64, conDetails *ibapi.ContractDetails) {
	// w.stdout.Println("<ContractDetails>")
	channel, _ := w.responseData.Load(reqID)
	channel.(chan interface{}) <- conDetails
}

func (w *WrapperChannel) ContractDetailsEnd(reqID int64) {
	// w.stdout.Println("<ContractDetailsEnd>")
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

func (w *WrapperChannel) BondContractDetails(reqID int64, conDetails *ibapi.ContractDetails) {
	w.stdout.Println("<BondContractDetails>")
}

func (w *WrapperChannel) SymbolSamples(reqID int64, contractDescriptions []ibapi.ContractDescription) {
	w.stdout.Println("<SymbolSamples>")
}

func (w *WrapperChannel) SmartComponents(reqID int64, smartComps []ibapi.SmartComponent) {
	w.stdout.Println("<SmartComponents>")
}

func (w *WrapperChannel) MarketRule(marketRuleID int64, priceIncrements []ibapi.PriceIncrement) {
	w.stdout.Println("<MarketRule>")
}

func (w *WrapperChannel) RealtimeBar(reqID int64, t int64, open float64, high float64, low float64, close float64, volume int64, wap float64, count int64) {
	w.stdout.Printf("<RealtimeBar> %v %v ", time.Unix(t, 0).String(), low)
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

func (w *WrapperChannel) HistoricalData(reqID int64, bar *ibapi.BarData) {
	w.stdout.Println("<HistoricalData>")
}

func (w *WrapperChannel) HistoricalDataEnd(reqID int64, startDateStr string, endDateStr string) {
	w.stdout.Println("<HistoricalDataEnd>")
}

func (w *WrapperChannel) HistoricalDataUpdate(reqID int64, bar *ibapi.BarData) {
	w.stdout.Println("<HistoricalDataUpdate>")
}

func (w *WrapperChannel) HeadTimestamp(reqID int64, headTimestamp string) {
	w.stdout.Println("<HeadTimestamp>")
}

func (w *WrapperChannel) HistoricalTicks(reqID int64, ticks []ibapi.HistoricalTick, done bool) {
	w.stdout.Println("<HistoricalTicks>")
}

func (w *WrapperChannel) HistoricalTicksBidAsk(reqID int64, ticks []ibapi.HistoricalTickBidAsk, done bool) {
	w.stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}

func (w *WrapperChannel) HistoricalTicksLast(reqID int64, ticks []ibapi.HistoricalTickLast, done bool) {
	w.stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}

func (w *WrapperChannel) TickSize(reqID int64, tickType int64, size int64) {
	w.stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}

func (w *WrapperChannel) TickSnapshotEnd(reqID int64) {
	w.stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}

func (w *WrapperChannel) MarketDataType(reqID int64, marketDataType int64) {
	w.stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}

func (w *WrapperChannel) TickByTickAllLast(reqID int64, tickType int64, time int64, price float64, size int64, tickAttribLast ibapi.TickAttribLast, exchange string, specialConditions string) {
	w.stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}

func (w *WrapperChannel) TickByTickBidAsk(reqID int64, time int64, bidPrice float64, askPrice float64, bidSize int64, askSize int64, tickAttribBidAsk ibapi.TickAttribBidAsk) {
	w.stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}

func (w *WrapperChannel) TickByTickMidPoint(reqID int64, time int64, midPoint float64) {
	w.stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}

func (w *WrapperChannel) TickString(reqID int64, tickType int64, value string) {
	w.stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}

func (w *WrapperChannel) TickGeneric(reqID int64, tickType int64, value float64) {
	w.stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}

func (w *WrapperChannel) TickEFP(reqID int64, tickType int64, basisPoints float64, formattedBasisPoints string, totalDividends float64, holdDays int64, futureLastTradeDate string, dividendImpact float64, dividendsToLastTradeDate float64) {
	w.stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}

func (w *WrapperChannel) TickReqParams(reqID int64, minTick float64, bboExchange string, snapshotPermissions int64) {
	w.stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}
func (w *WrapperChannel) MktDepthExchanges(depthMktDataDescriptions []ibapi.DepthMktDataDescription) {
	w.stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
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
func (w *WrapperChannel) UpdateMktDepth(reqID int64, position int64, operation int64, side int64, price float64, size int64) {
	w.stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}

func (w *WrapperChannel) UpdateMktDepthL2(reqID int64, position int64, marketMaker string, operation int64, side int64, price float64, size int64, isSmartDepth bool) {
	w.stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}

func (w *WrapperChannel) TickOptionComputation(reqID int64, tickType int64, impliedVol float64, delta float64, optPrice float64, pvDiviedn float64, gamma float64, vega float64, theta float64, undPrice float64) {
	w.stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}

func (w *WrapperChannel) FundamentalData(reqID int64, data string) {
	w.stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}

func (w *WrapperChannel) ScannerParameters(xml string) {
	w.stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}

func (w *WrapperChannel) ScannerData(reqID int64, rank int64, conDetails *ibapi.ContractDetails, distance string, benchmark string, projection string, legs string) {
	w.stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}

func (w *WrapperChannel) ScannerDataEnd(reqID int64) {
	w.stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}

func (w *WrapperChannel) HistogramData(reqID int64, histogram []ibapi.HistogramData) {
	w.stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}

func (w *WrapperChannel) RerouteMktDataReq(reqID int64, contractID int64, exchange string) {
	w.stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}

func (w *WrapperChannel) RerouteMktDepthReq(reqID int64, contractID int64, exchange string) {
	w.stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}

func (w *WrapperChannel) SecurityDefinitionOptionParameter(reqID int64, exchange string, underlyingContractID int64, tradingClass string, multiplier string, expirations []string, strikes []float64) {
	w.stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}

func (w *WrapperChannel) SecurityDefinitionOptionParameterEnd(reqID int64) {
	w.stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}

func (w *WrapperChannel) SoftDollarTiers(reqID int64, tiers []ibapi.SoftDollarTier) {
	w.stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}

func (w *WrapperChannel) FamilyCodes(famCodes []ibapi.FamilyCode) {
	w.stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}

func (w *WrapperChannel) NewsProviders(newsProviders []ibapi.NewsProvider) {
	w.stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}

func (w *WrapperChannel) TickNews(tickerID int64, timeStamp int64, providerCode string, articleID string, headline string, extraData string) {
	w.stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}

func (w *WrapperChannel) NewsArticle(reqID int64, articleType int64, articleText string) {
	w.stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}

func (w *WrapperChannel) HistoricalNews(reqID int64, time string, providerCode string, articleID string, headline string) {
	w.stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}

func (w *WrapperChannel) HistoricalNewsEnd(reqID int64, hasMore bool) {
	w.stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}

func (w *WrapperChannel) UpdateNewsBulletin(msgID int64, msgType int64, newsMessage string, originExch string) {
	w.stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}

func (w *WrapperChannel) ReceiveFA(faData int64, cxml string) {
	w.stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}

func (w *WrapperChannel) CurrentTime(t time.Time) {
	w.stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
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

	w.stdout.Printf("[ibapi] (%v) ERROR %v - %s", reqID, errCode, errString)
}

func (w *WrapperChannel) CompletedOrder(contract *ibapi.Contract, order *ibapi.Order, orderState *ibapi.OrderState) {
	w.stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}

func (w *WrapperChannel) CompletedOrdersEnd() {
	w.stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}

func (w *WrapperChannel) ReplaceFAEnd(reqID int64, text string) {
	w.stdout.Fatal("WRAPPER FUNCTION NOT IMPLEMENTED")
}
