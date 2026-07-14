.PHONY: build test lint install clean

build:
	go build -o bin/mt5-backtester-pp-cli ./cmd/mt5-backtester-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/mt5-backtester-pp-cli

clean:
	rm -rf bin/
