package gotrader

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"testing"
	"time"
)

func TestBacktestBrocker_TestOrders(t *testing.T) {
	t.Skip("partially filled orders have been disabled in backtesting :(")
	t.Parallel()

	broker := BacktestBrocker{
		BrokerAvailableCash: 30000,
		OrderMap:            map[string]*Order{},
		Portfolio:           map[Symbol]Position{},
		EvalCommissions:     Nocommissions,
	}

	_, err := broker.GetOrderByID("order that does not exists")
	if !errors.Is(err, ErrOrderNotFound) {
		t.Fatal("Expected an error if the order is not found")
	}

	orderIdAmzn, err := broker.SubmitOrder(Candle{}, Order{
		Id:     "this will be changed",
		Size:   178,
		Symbol: "AMZN",
		Type:   OrderBuy,
	})
	if orderIdAmzn == "this will be changed" {
		t.Fatal("orderId has not been override for AMZN")
	}

	orderIdTsla, err := broker.SubmitOrder(Candle{}, Order{
		Id:     "this will be changed",
		Size:   456,
		Symbol: "TSLA",
		Type:   OrderSell,
	})
	if orderIdTsla == "this will be changed" {
		t.Fatal("orderId has not been override for TSLA")
	}

	amzn, err := broker.GetOrderByID(orderIdAmzn)
	if err != nil {
		t.Fatal(err)
	}

	if amzn.Symbol != "AMZN" {
		t.Fatalf("expected AMZN, got %s", amzn.Symbol)
	}

	if amzn.Status != OrderStatusAccepted {
		t.Fatalf("expected order in StautsAccepted, got %v", amzn.Status)
	}

	// Test a partially fulfilled order //
	broker.ProcessOrders(Candle{
		Open:   100,
		High:   110,
		Close:  101,
		Low:    90,
		Volume: 170,
		Symbol: "AMZN",
		Time:   time.Time{},
	})

	amznSemiFullfilled, err := broker.GetOrderByID(orderIdAmzn)
	if err != nil {
		t.Fatal(err)
	}

	if amznSemiFullfilled.Status != OrderStatusPartiallyFilled {
		t.Fatal("OrderStatus mismatch")
	}

	if amznSemiFullfilled.SizeFilled != 170 {
		t.Fatal("Partial execution mismatch")
	}

	broker.ProcessOrders(Candle{
		Open:   1024,
		High:   110,
		Close:  101,
		Low:    90,
		Volume: 200,
		Symbol: "AMZN",
		Time:   time.Time{},
	})

	amznDone, err := broker.GetOrderByID(orderIdAmzn)
	if err != nil {
		t.Fatal(err)
	}

	if amznDone.Status != OrderStatusFullFilled {
		t.Fatal("OrderStatus mismatch")
	}

	if amznDone.SizeFilled != 178 {
		t.Fatal("Order not fulfilled?")
	}

	position := broker.GetPosition("AMZN")
	if !almostEqual(position.AvgPrice, 141.52) {
		t.Fatalf("expected avg price was 141.52, got %v", position.AvgPrice)
	}
}

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) <= 1e-2
}

type OllamaRequest struct {
	Model        string             `json:"model"`
	Prompt       string             `json:"prompt"`
	Stream       bool               `json:"stream"`
	KeepAlive    string             `json:"keep_alive"`
	ModelOptions OllamaModelOptions `json:"options"`
}

type OllamaModelOptions struct {
	NumCtx int `json:"num_ctx"`
}

type OllamaResponse struct {
	Model              string    `json:"model"`
	CreatedAt          time.Time `json:"created_at"`
	Response           string    `json:"response"`
	Done               bool      `json:"done"`
	DoneReason         string    `json:"done_reason"`
	Context            []int     `json:"context"`
	TotalDuration      int64     `json:"total_duration"`
	LoadDuration       int64     `json:"load_duration"`
	PromptEvalCount    int       `json:"prompt_eval_count"`
	PromptEvalDuration int64     `json:"prompt_eval_duration"`
	EvalCount          int       `json:"eval_count"`
	EvalDuration       int       `json:"eval_duration"`
}

//go:embed prova.prompt
var prompt string

func TestOllama(t *testing.T) {
	for i := 0; i < 10; i++ {
		_, err := doReq()
		if err != nil {
			t.Fatal(err)
		}

		// println(action)
	}
}
func doReq() (string, error) {
	request := OllamaRequest{
		Model: "deepseek-r1:32b",
		// Model:     "deepseek-coder-v2",  // 80% hold, veloce? da provare ocn prompt diversi
		Prompt:    prompt,
		Stream:    false,
		KeepAlive: "5m",
		// ModelOptions: OllamaModelOptions{
		// 	NumCtx: 32000,
		// },
	}

	payload, err := json.Marshal(request)
	if err != nil {
		return "", err
	}

	start := time.Now()
	response, err := http.Post("http://192.168.188.78:11434/api/generate", "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return "", err
	}
	elapsed := time.Since(start)

	// println(response.StatusCode)

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}
	// println(string(responseBody))

	var action OllamaResponse
	err = json.Unmarshal(responseBody, &action)
	if err != nil {
		return "", err
	}
	println(fmt.Sprintf("action=%s elapsed=%v", action.Response, elapsed))
	return action.Response, nil
}
