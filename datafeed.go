package gotrader

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Candle struct {
	Open   float64
	High   float64
	Close  float64
	Low    float64
	Volume int64
	Symbol Symbol
	Time   time.Time
}

func (candle Candle) TimeStr() string {
	//return fmt.Sprintf("%v [%s]", candle.Time.Format("2006-01-02 15:04:05"), candle.Symbol)
	return fmt.Sprintf(" %-5s %v", candle.Symbol, candle.Time.Format("15:04:05"))
}

func (candle Candle) String() string {
	return fmt.Sprintf("[%-5s %v] open:%v high:%v close:%v low:%v volume:%v", candle.Symbol, candle.Time.Format("15:04:05"), candle.Open, candle.High, candle.Close, candle.Low, candle.Volume)
}

// DataFeed provides a stream of Candle.
type DataFeed interface {

	// Run starts a go routine that poll the data source, and push the candles in the returned channel.
	// The channel is expected to have a buffer larger enough to handle 1 day of data
	Run() (chan Candle, error)
}

// <editor-fold desc="IBZippedCSV" >

type IBZippedCSV struct {
	DataFolder string
	Sday       time.Time
	Symbol     Symbol
}

func (d *IBZippedCSV) Run() (chan Candle, error) {

	file := filepath.Join(d.DataFolder, fmt.Sprintf("%s-%s.csv", d.Sday.Format("20060102"), d.Symbol))
	log.Printf("opening file %s", file)

	f, err := os.Open(file)
	if err != nil {

		// When running tests from the IDE, the working dir is in the folder of the test file.
		// This porkaround allow us to easily run tests
		file = filepath.Join("..", d.DataFolder, fmt.Sprintf("%s-%s.csv", d.Sday.Format("20060102"), d.Symbol))
		log.Printf("opening file - retrying %s", file)
		f, err = os.Open(file)
		if err != nil {
			return nil, err
		}
	}

	stream := make(chan Candle, 24*time.Hour/time.Second)
	log.Println("Start feeding the candles in the channel")

	go func() {
		scanner := bufio.NewScanner(f)
		defer f.Close()

		latestInst := time.Date(1984, 5, 8, 4, 32, 19, 0, time.Local)
		for scanner.Scan() {
			parts := strings.Split(scanner.Text(), ",")
			inst, err := time.ParseInLocation("20060102 15:04:05", parts[0], time.Local)
			if err != nil {
				log.Println("[ERROR] Can't parse the datetime! Skipping a candle")
				continue
			}

			// Skip candles that are in the past (should never happen, but happened with IB csv files)
			if inst.Before(latestInst) || inst.Equal(latestInst) {
				log.Printf("[WARNING] skipping candle in the past! Last: %v, new:%v", latestInst.String(), inst.String())
				continue
			}
			latestInst = inst

			candle := Candle{
				Symbol: d.Symbol,
				Time:   inst,
				Open:   mustFloat(parts[1]),
				High:   mustFloat(parts[2]),
				Low:    mustFloat(parts[3]),
				Close:  mustFloat(parts[4]),
				Volume: mustInt(parts[5]),
			}
			stream <- candle
		}

		log.Println("closing datafeed")
		close(stream)

	}()

	return stream, nil
}

func mustFloat(str string) float64 {
	n, err := strconv.ParseFloat(str, 64)
	if err != nil {
		log.Fatalf("Cant parse the string %s to a float64 -- %v", str, err)
		return 0
	}
	return n
}

func mustInt(str string) int64 {
	n, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		log.Fatalf("Cant parse the string %s to a float64 -- %v", str, err)
		return 0
	}
	return n
}

// </editor-fold>
