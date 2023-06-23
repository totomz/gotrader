module github.com/totomz/gotrader

go 1.19

require (
	github.com/alpacahq/alpaca-trade-api-go/v2 v2.5.0
	github.com/cinar/indicator v1.2.18
	github.com/google/go-cmp v0.5.8
	github.com/hadrianl/ibapi v0.0.0-20210428041841-65ae418d9353
	github.com/joho/godotenv v1.4.0
	github.com/pkg/errors v0.8.1
	github.com/shopspring/decimal v1.3.1
	go.opencensus.io v0.23.0

	// required by ibapi :(
	go.uber.org/zap v1.16.0
)

require (
	github.com/fsnotify/fsnotify v1.6.0
	golang.org/x/exp v0.0.0-20230522175609-2e198f4a06a1
)

require (
	cloud.google.com/go v0.100.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	go.uber.org/atomic v1.7.0 // indirect
	go.uber.org/multierr v1.6.0 // indirect
	golang.org/x/sys v0.1.0 // indirect
)
