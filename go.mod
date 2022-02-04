module github.com/totomz/gotrader

go 1.16

require (
	cloud.google.com/go v0.100.2 // indirect
	github.com/alpacahq/alpaca-trade-api-go/v2 v2.2.0
	github.com/cinar/indicator v1.1.0
	github.com/google/go-cmp v0.5.6
	github.com/hadrianl/ibapi v0.0.0-20210428041841-65ae418d9353
	github.com/joho/godotenv v1.4.0
	github.com/pkg/errors v0.8.1
	github.com/shopspring/decimal v1.3.1

	// required by ibapi :(
	go.uber.org/zap v1.16.0
)
