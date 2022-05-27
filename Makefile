
COMMIT = $(shell git rev-parse HEAD | head -c 7)
TIMESTAMP=$(shell TZ='UTC' date '+%Y%m%dt%H%M')

build: get
	go vet
	go build -ldflags "-X main.build=$(TIMESTAMP)-$(COMMIT)" -o server .

get:
	go get -t -v 

test: get build		
	go test -parallel 8 -count=1 -cover $$(go list ./... | grep -v /interactivebrokers)

test-all: get build		
	go test -parallel 8 -count=1 -cover ./...

build-gotaset:
	go vet
	GOOS=linux go build -ldflags "-X main.build=$(TIMESTAMP)-$(COMMIT)" -o bin/gotaset_linux gotaset/app/main.go
	GOOS=darwin go build -ldflags "-X main.build=$(TIMESTAMP)-$(COMMIT)" -o bin/gotaset_darwin gotaset/app/main.go

deploy-gotaset: 
	cd gotaset
	docker build -t pippo .