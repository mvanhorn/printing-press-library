.PHONY: build test lint install clean

build:
	go build -o bin/NSE India-pp-cli ./cmd/NSE India-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/NSE India-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/NSE India-pp-mcp ./cmd/NSE India-pp-mcp

install-mcp:
	go install ./cmd/NSE India-pp-mcp

build-all: build build-mcp
