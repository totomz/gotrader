package gotrader

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"
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
	Slowtime   time.Duration
	Symbol     Symbol
	Symbols    []Symbol
}

func (d *IBZippedCSV) Run() (chan Candle, error) {
	var files []*os.File
	var scanners []*bufio.Scanner
	var latestInsts []time.Time

	stream := make(chan Candle, 24*time.Hour/time.Second)
	Stdout.Println("Start feeding the candles in the channel")

	if len(d.Symbols) == 0 {
		d.Symbols = []Symbol{d.Symbol}
	}

	for _, s := range d.Symbols {
		file := filepath.Join(d.DataFolder, fmt.Sprintf("%s-%s.csv", d.Sday.Format("20060102"), s))
		Stdout.Printf("opening file %s", file)

		f, err := os.Open(file)

		if err != nil {
			// When running tests from the IDE, the working dir is in the folder of the test file.
			// This porkaround allow us to easily run tests
			file = filepath.Join("..", d.DataFolder, fmt.Sprintf("%s-%s.csv", d.Sday.Format("20060102"), s))
			Stdout.Printf("opening file - retrying %s", file)
			f, err = os.Open(file)
			if err != nil {
				return nil, err
			}
		}

		files = append(files, f)
		scanners = append(scanners, bufio.NewScanner(f))
		latestInsts = append(latestInsts, time.Date(1984, 5, 8, 4, 32, 19, 0, time.Local))
	}

	go func() {

		openScanners := len(scanners)

		for {
			if openScanners == 0 {
				break
			}

			for i, scanner := range scanners {

				if !scanner.Scan() {
					_ = files[i].Close()
					openScanners -= 1
					continue
				}

				parts := strings.Split(scanner.Text(), ",")
				inst, err := time.ParseInLocation("20060102 15:04:05", parts[0], time.Local)
				if err != nil {
					Stderr.Println("Can't parse the datetime! Skipping a candle")
					continue
				}

				// Skip candles that are in the past (should never happen, but happened with IB csv files)
				if inst.Before(latestInsts[i]) || inst.Equal(latestInsts[i]) {
					Stdout.Printf("skipping candle in the past! Last: %v, new:%v", latestInsts[i].String(), inst.String())
					continue
				}
				latestInsts[i] = inst

				candle := Candle{
					Symbol: d.Symbols[i],
					Time:   inst,
					Open:   mustFloat(parts[1]),
					High:   mustFloat(parts[2]),
					Low:    mustFloat(parts[3]),
					Close:  mustFloat(parts[4]),
					Volume: mustInt(parts[5]),
				}
				stream <- candle

			}

			if d.Slowtime > 0 {
				time.Sleep(d.Slowtime)
			}
		}

		Stdout.Println("closing datafeed")
		close(stream)

	}()

	return stream, nil
}

func mustFloat(str string) float64 {
	n, err := strconv.ParseFloat(str, 64)
	if err != nil {
		log.Fatalf("Can't parse the string '%s' to a float64 -- %v", str, err)
	}
	return n
}

func mustInt(str string) int64 {
	n, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		log.Fatalf("Can't parse the string %s to an int64 -- %v", str, err)
	}
	return n
}

// </editor-fold>

var (
	CsvIndexOpen   = 1
	CsvIndexHigh   = 2
	CsvIndexLow    = 3
	CsvIndexClose  = 4
	CsvIndexVolume = 5
	CsvIndexTime   = 6
)

type ZippedCSV struct {
	DataFolder string
	Sday       time.Time
	Slowtime   time.Duration
	Symbol     Symbol
	Symbols    []Symbol
}

func (d *ZippedCSV) Run() (chan Candle, error) {
	var files []*os.File
	var scanners []*bufio.Scanner
	var readers []*gzip.Reader
	var latestInsts []time.Time

	stream := make(chan Candle, 24*time.Hour/time.Second)
	Stdout.Println("Start feeding the candles in the channel")

	if len(d.Symbols) == 0 {
		d.Symbols = []Symbol{d.Symbol}
	}

	for _, s := range d.Symbols {
		file := filepath.Join(d.DataFolder, fmt.Sprintf("%s-%s.csv.gz", d.Sday.Format("20060102"), s))
		Stdout.Printf("opening file %s", file)

		f, err := os.Open(file)

		if err != nil {
			// When running tests from the IDE, the working dir is in the folder of the test file.
			// This porkaround allow us to easily run tests
			file = filepath.Join("..", d.DataFolder, fmt.Sprintf("%s-%s.csv", d.Sday.Format("20060102"), s))
			Stdout.Printf("opening file - retrying %s", file)
			f, err = os.Open(file)
			if err != nil {
				return nil, err
			}
		}

		reader, err := gzip.NewReader(f)
		if err != nil {
			panic(err)
		}

		files = append(files, f)
		readers = append(readers, reader)
		scanners = append(scanners, bufio.NewScanner(reader))
		latestInsts = append(latestInsts, time.Date(1984, 5, 8, 4, 32, 19, 0, time.Local))
	}

	go func() {
		openScanners := len(scanners)

		for {
			if openScanners == 0 {
				break
			}

			for i, scanner := range scanners {

				if !scanner.Scan() {
					_ = readers[i].Close()
					_ = files[i].Close()
					openScanners -= 1
					continue
				}

				line := scanner.Text()
				if !unicode.IsDigit(rune(line[0])) {
					parts := strings.Split(line, ",")
					for i, p := range parts {
						switch p {
						case "open":
							CsvIndexOpen = i
						case "high":
							CsvIndexHigh = i
						case "close":
							CsvIndexClose = i
						case "low":
							CsvIndexLow = i
						case "volume":
							CsvIndexVolume = i
						case "timestamp":
							CsvIndexTime = i
						}
					}
					continue
				}

				parts := strings.Split(line, ",")
				inst, err := time.ParseInLocation("2006-01-02 15:04:05-0700", parts[CsvIndexTime], time.Local)
				if err != nil {
					Stderr.Println("Can't parse the datetime! Skipping a candle")
					continue
				}

				// Skip candles that are in the past (should never happen, but happened with IB csv files)
				if inst.Before(latestInsts[i]) || inst.Equal(latestInsts[i]) {
					Stdout.Printf("skipping candle in the past! Last: %v, new:%v", latestInsts[i].String(), inst.String())
					continue
				}
				latestInsts[i] = inst

				candle := Candle{
					Symbol: d.Symbols[i],
					Time:   inst,
					Open:   mustFloat(parts[CsvIndexOpen]),
					High:   mustFloat(parts[CsvIndexHigh]),
					Low:    mustFloat(parts[CsvIndexLow]),
					Close:  mustFloat(parts[CsvIndexClose]),
					Volume: mustInt(parts[CsvIndexVolume]),
				}
				stream <- candle

			}

			if d.Slowtime > 0 {
				time.Sleep(d.Slowtime)
			}
		}

		Stdout.Println("closing datafeed")
		close(stream)

	}()

	return stream, nil
}
