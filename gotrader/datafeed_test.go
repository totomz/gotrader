package gotrader

import (
	"log"
	"testing"
	"time"
)

func init() {
	log.SetFlags(log.Ltime | log.Lshortfile)
}

var testSday = time.Date(2021, 1, 11, 0, 0, 0, 0, time.Local)

const testSymbol = Symbol("FB")
const testFolder = "datasets"

func TestIBZippedCSV_ReadFile(t *testing.T) {
	datafeed := IBZippedCSV{
		DataFolder: testFolder,
		Sday:       testSday,
		Symbol:     testSymbol,
	}

	input, err := datafeed.Run()
	if err != nil {
		t.Fatalf("Error reading CSV file -- %v", err)
	}

	var candles []Candle
	for candle := range input {
		candles = append(candles, candle)
	}

	// How many lines? The csv has 1 line for each second.
	// How many seconds in the time interval?
	a := time.Date(2021, 6, 15, 15, 30, 00, 00, time.Local)
	b := time.Date(2021, 6, 15, 21, 59, 59, 00, time.Local)
	rows := b.Sub(a).Seconds() + 1 // +1 because seconds starts at 0, line count at 1

	if len(candles) != int(rows) {
		t.Fatalf("Expected 25200 candles got %v", len(candles))
	}

	if candles[0].Time.Add(3*time.Second).Second() != 3 {
		t.Fatalf("Expected 3 seconds - random test")
	}

}

func TestIBZippedCSV_ReadFileIgnoreDuplicatesLines(t *testing.T) {
	datafeed := IBZippedCSV{
		DataFolder: testFolder,
		Sday:       testSday,
		Symbol:     "FBDUPLICATES",
	}

	input, err := datafeed.Run()
	if err != nil {
		t.Fatalf("Error reading CSV file -- %v", err)
	}

	var candles []Candle
	for candle := range input {
		candles = append(candles, candle)
	}

	// How many lines? The csv has 1 line for each second.
	// How many seconds in the time interval?
	a := time.Date(2021, 6, 15, 15, 30, 00, 00, time.Local)
	b := time.Date(2021, 6, 15, 21, 59, 59, 00, time.Local)
	rows := b.Sub(a).Seconds() + 1 // +1 because seconds starts at 0, line count at 1

	if len(candles) != int(rows) {
		t.Fatalf("Expected %v candles got %v", rows, len(candles))
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
