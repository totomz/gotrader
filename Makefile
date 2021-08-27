
ROOT=$(shell git rev-parse --show-toplevel)


build: get
	go vet
	go build -ldflags "-X main.build=$(TIMESTAMP)-$(COMMIT)" -o server .

get:
	go get -t -v 

test: get build		
	go test -parallel 8 -count=1 -cover $$(go list ./... | grep -v /interactivebrokers)
	#go test -parallel 8 -count=1 -cover ./...

test-all: get build		
	go test -parallel 8 -count=1 -cover ./...
