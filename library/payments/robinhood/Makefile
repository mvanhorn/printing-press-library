.PHONY: build test lint install clean

build:
	go build -o bin/robinhood-pp-cli ./cmd/robinhood-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/robinhood-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/robinhood-pp-mcp ./cmd/robinhood-pp-mcp

install-mcp:
	go install ./cmd/robinhood-pp-mcp

build-all: build build-mcp
