package gotrader

import (
	"bufio"
	"compress/gzip"
	"errors"
	"fmt"
	"github.com/parquet-go/parquet-go"
	"golang.org/x/exp/slog"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
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

var locationNewYork = sync.OnceValue[*time.Location](func() *time.Location {
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		panic(err)
	}
	return location
})

func (candle Candle) TimeStr() string {
	return fmt.Sprintf("%-5s %v", candle.Symbol, candle.Time.Format("15:04:05"))
}

func (candle Candle) String() string {
	return fmt.Sprintf("[%-5s %v] open:%v high:%v close:%v low:%v volume:%v", candle.Symbol, candle.Time.In(locationNewYork()).Format("15:04:05"), candle.Open, candle.High, candle.Close, candle.Low, candle.Volume)
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
	slog.Info("Start feeding the candles in the channel")

	if len(d.Symbols) == 0 {
		d.Symbols = []Symbol{d.Symbol}
	}

	for _, s := range d.Symbols {
		file := filepath.Join(d.DataFolder, fmt.Sprintf("%s-%s.csv", d.Sday.Format("20060102"), s))
		slog.Info("opening file %s", file)

		f, err := os.Open(file)

		if err != nil {
			// When running tests from the IDE, the working dir is in the folder of the test file.
			// This porkaround allow us to easily run tests
			file = filepath.Join("..", d.DataFolder, fmt.Sprintf("%s-%s.csv", d.Sday.Format("20060102"), s))
			slog.Info("opening file - retrying %s", file)
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
					slog.Error("Can't parse the datetime! Skipping a candle")
					continue
				}

				// Skip candles that are in the past (should never happen, but happened with IB csv files)
				if inst.Before(latestInsts[i]) || inst.Equal(latestInsts[i]) {
					slog.Info("skipping candle in the past!", "last", latestInsts[i].String(), "new", inst.String())
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

		// slog.Info("closing datafeed")
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
	slog.Info("Start feeding the candles in the channel")

	if len(d.Symbols) == 0 {
		d.Symbols = []Symbol{d.Symbol}
	}

	for _, s := range d.Symbols {
		file := filepath.Join(d.DataFolder, fmt.Sprintf("%s-%s.csv.gz", d.Sday.Format("20060102"), s))
		slog.Info("opening file", "file", file)

		f, err := os.Open(file)

		if err != nil {
			// When running tests from the IDE, the working dir is in the folder of the test file.
			// This porkaround allow us to easily run tests
			file = filepath.Join("..", d.DataFolder, fmt.Sprintf("%s-%s.csv", d.Sday.Format("20060102"), s))
			slog.Info("opening file - retrying", "file", file)
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
				inst, err := time.ParseInLocation("2006-01-02 15:04:05-07:00", parts[CsvIndexTime], time.Local)
				if err != nil {
					slog.Error("Can't parse the datetime! Skipping a candle")
					continue
				}

				// Skip candles that are in the past (should never happen, but happened with IB csv files)
				if inst.Before(latestInsts[i]) || inst.Equal(latestInsts[i]) {
					slog.Error("skipping candle in the past!", "last", latestInsts[i].String(), "new", inst.String())
					continue
				}

				if !IsNasdaqTradingTime(inst) {
					// slog.Error("not in NASDAQ trading time", "inst", inst.String())
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

		// slog.Info("closing datafeed")
		close(stream)

	}()

	return stream, nil
}

var getNyTimeZone = sync.OnceValue[*time.Location](func() *time.Location {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		panic(err)
	}

	return loc
})

func IsNasdaqTradingTime(t time.Time) bool {
	// Load New York timezone

	// Convert UTC time to New York time
	tInNY := t.In(getNyTimeZone())

	// Check if it's a weekday (Monday=1, ..., Friday=5)
	weekday := tInNY.Weekday()
	if weekday < time.Monday || weekday > time.Friday {
		slog.Info("time outside NASDAW trading hours")
		return false
	}

	// Get the time components
	hour := tInNY.Hour()
	minute := tInNY.Minute()

	// Market opens at 9:30 AM ET
	marketOpen := hour > 9 || (hour == 9 && minute >= 30)

	// Market closes at 4:00 PM ET (16:00)
	marketClose := hour < 16

	return marketOpen && marketClose
}

// <editor-fold desc="GenericParquetTrades" >

const (
	parquetKindTrades    = "trades"
	parquetKindQuotes    = "quotes"
	parquetDayLayout     = "2006-01-02"
	parquetReadBatchSize = 1024
)

type parquetTradeRow struct {
	Ticker        string  `parquet:"ticker"`
	TS            int64   `parquet:"ts"`
	ParticipantTS int64   `parquet:"participant_ts"`
	Seq           int64   `parquet:"seq"`
	Price         float64 `parquet:"price"`
	Size          int64   `parquet:"size"`
	Exchange      int32   `parquet:"exchange"`
	Tape          int32   `parquet:"tape"`
	Conditions    []int32 `parquet:"conditions,list"`
	UpdatesLast   bool    `parquet:"updates_last"`
	UpdatesVolume bool    `parquet:"updates_volume"`
	IsRTH         bool    `parquet:"is_rth"`
}

func parquetPathFor(kind, dataFolder string, day time.Time, ticker Symbol) string {
	return filepath.Join(dataFolder, kind, day.Format(parquetDayLayout), fmt.Sprintf("%s.parquet", ticker))
}

func tradeFromParquetRow(row parquetTradeRow) Trade {
	trade := Trade{
		Ticker:        Symbol(row.Ticker),
		TS:            row.TS,
		ParticipantTS: row.ParticipantTS,
		Seq:           row.Seq,
		Price:         row.Price,
		Size:          row.Size,
		Exchange:      int16(row.Exchange),
		Tape:          int8(row.Tape),
		UpdatesLast:   row.UpdatesLast,
		UpdatesVolume: row.UpdatesVolume,
		IsRTH:         row.IsRTH,
	}

	if len(row.Conditions) > 0 {
		trade.Conditions = make([]int16, len(row.Conditions))
		for i, condition := range row.Conditions {
			trade.Conditions[i] = int16(condition)
		}
	}

	return trade
}

func tradeToParquetRow(trade Trade) parquetTradeRow {
	row := parquetTradeRow{
		Ticker:        string(trade.Ticker),
		TS:            trade.TS,
		ParticipantTS: trade.ParticipantTS,
		Seq:           trade.Seq,
		Price:         trade.Price,
		Size:          trade.Size,
		Exchange:      int32(trade.Exchange),
		Tape:          int32(trade.Tape),
		UpdatesLast:   trade.UpdatesLast,
		UpdatesVolume: trade.UpdatesVolume,
		IsRTH:         trade.IsRTH,
	}

	if len(trade.Conditions) > 0 {
		row.Conditions = make([]int32, len(trade.Conditions))
		for i, condition := range trade.Conditions {
			row.Conditions[i] = int32(condition)
		}
	}

	return row
}

// readParquetTrades streams one trades file in out, one row group at a time, without
// loading the whole file in memory. Rows are delivered as-is, with no filtering: the only
// check is the (ts, seq) monotonicity, an out of order row is skipped and never re-sorted.
// The caller owns out and is responsible for closing it.
func readParquetTrades(path string, out chan<- Trade) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("can not open the trades file %s: %w", path, err)
	}
	defer func() { _ = file.Close() }()

	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("can not stat the trades file %s: %w", path, err)
	}

	parquetFile, err := parquet.OpenFile(file, info.Size())
	if err != nil {
		return fmt.Errorf("can not read the trades file %s: %w", path, err)
	}

	rows := make([]parquetTradeRow, parquetReadBatchSize)
	lastTS := int64(0)
	lastSeq := int64(0)
	isFirst := true

	for _, rowGroup := range parquetFile.RowGroups() {
		reader := parquet.NewGenericRowGroupReader[parquetTradeRow](rowGroup)

		for {
			read, readErr := reader.Read(rows)

			for i := 0; i < read; i++ {
				trade := tradeFromParquetRow(rows[i])

				if !isFirst && (trade.TS < lastTS || (trade.TS == lastTS && trade.Seq < lastSeq)) {
					slog.Error("skipping an out of order trade", "file", path, "ticker", trade.Ticker,
						"ts", trade.TS, "seq", trade.Seq, "last_ts", lastTS, "last_seq", lastSeq)
					continue
				}

				lastTS = trade.TS
				lastSeq = trade.Seq
				isFirst = false
				out <- trade
			}

			if errors.Is(readErr, io.EOF) {
				break
			}

			if readErr != nil {
				_ = reader.Close()
				return fmt.Errorf("can not read the trades file %s: %w", path, readErr)
			}
		}

		if err = reader.Close(); err != nil {
			return fmt.Errorf("can not close the reader of the trades file %s: %w", path, err)
		}
	}

	return nil
}

// </editor-fold>
