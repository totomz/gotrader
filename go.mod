module github.com/totomz/gotrader

go 1.24.0

require (
	github.com/alpacahq/alpaca-trade-api-go/v2 v2.5.0
	github.com/google/go-cmp v0.7.0
	github.com/hadrianl/ibapi v0.0.0-20210428041841-65ae418d9353
	github.com/joho/godotenv v1.4.0
	github.com/pkg/errors v0.8.1
	github.com/shopspring/decimal v1.4.0
	go.opencensus.io v0.24.0

	// required by ibapi :(
	go.uber.org/zap v1.16.0
)

require (
	github.com/fsnotify/fsnotify v1.6.0
	github.com/redis/rueidis v1.0.9
	golang.org/x/exp v0.0.0-20230522175609-2e198f4a06a1
)

require (
	cloud.google.com/go v0.123.0 // indirect
	github.com/alpacahq/alpaca-trade-api-go/v3 v3.9.1 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/mailru/easyjson v0.9.2 // indirect
	go.uber.org/atomic v1.7.0 // indirect
	go.uber.org/multierr v1.6.0 // indirect
	golang.org/x/sys v0.35.0 // indirect
)
