.PHONY: build test lint install clean

build:
	go build -o bin/revenuecat-pp-cli ./cmd/revenuecat-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/revenuecat-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/revenuecat-pp-mcp ./cmd/revenuecat-pp-mcp

install-mcp:
	go install ./cmd/revenuecat-pp-mcp

build-all: build build-mcp
