package gotrader

import (
	"log"
	"testing"
	"time"
)

func init() {
	log.SetFlags(log.Ltime | log.Lshortfile)
}

func TestIBZippedCSV_ReadFile(t *testing.T) {
	datafeed := IBZippedCSV{
		DataFolder: "datasets",
		Sday:       time.Date(2021, 1, 11, 0, 0, 0, 0, time.Local),
		Symbol:     "FB",
	}

	input, err := datafeed.Run()
	if err != nil {
		t.Fatalf("Error reading CSV file -- %v", err)
	}

	var candles []Candle
	for candle := range input {
		candles = append(candles, candle)
	}

	if len(candles) != 25200 {
		t.Fatalf("Expected 25200 candles got %v", len(candles))
	}

	if candles[0].Time.Add(3*time.Second).Second() != 3 {
		t.Fatalf("Expected 3 seconds - random test")
	}

}

func TestIBZippedCSVInvalidFile(t *testing.T) {

	datafeed := IBZippedCSV{
		Sday:   time.Date(2021, 1, 11, 0, 0, 0, 0, time.Local),
		Symbol: "NXNX",
	}

	input, err := datafeed.Run()

	if err == nil {
		t.Fatal("Expected an error")
	}

	isOpen := false
	select {
	case _, isOpen = <-input:
	default:
	}

	if isOpen {
		t.Fatalf("The candle channel should be closed")
	}
}
