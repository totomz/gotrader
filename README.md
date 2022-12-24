# go-trader
![build status](https://github.com/totomz/autotrader/actions/workflows/pipeline.yml/badge.svg)

go-trader is a platform to backtest and run live trading strategies, with focus on intra-day trading.

# Examples
Check out the [examples](examples)!

## Create a simple strategy
The full souce code for this example is in [examples/backtest/main.go](examples/backtest/main.go)
Import `gotrader` in your project
```shell
go get -v github.com/totomz/gotrader
```

Then write a simple strategy 
```go
package main
import "github.com/totomz/gotrader"

type SimpleStrategy struct {
	broker gotrader.Broker
}

func (s *SimpleStrategy) Initialize(cerbero *gotrader.Cerbero) {
	s.broker = cerbero.Broker
}

func (s *SimpleStrategy) Eval(candles []gotrader.Candle) {
	// c is the latest candle
	c := candles[len(candles)-1]
	println(c.String())
}
```

# Data Visualization
## Backtestsing
Candles and indicators are saved in `Signals`, that can be exported in a JSON format once the simulation is done
```go
dataGrafana := gotrader.SignalsToGrafana()
err = os.WriteFile("./plotly/signals_grafana.json", dataGrafana, 0644)
if err != nil {
    panic(err)
}
```

This file can be used to plot candles and indicators in Grafana


TODO:
- [ ] Visualizzo la roba su grafana
- [ ] BONUS: mi creo con le label "buy" and "sell"

# Credits
## Backtrader
go-trader is heavily inspired by [Backtrader](https://github.com/mementum/backtrader) 

## Indicators
The indicators are based on [github.com/cinar/indicator](https://github.com/cinar/indicator)

# Development
[shMake](https://github.com/totomz/shmake) is required to build and run the testss