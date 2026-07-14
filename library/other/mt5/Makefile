.PHONY: build test lint install clean build-mcp install-mcp build-all

build:
	go build -o bin/mt5-pp-cli ./cmd/mt5-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/mt5-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/mt5-pp-mcp ./cmd/mt5-pp-mcp

install-mcp:
	go install ./cmd/mt5-pp-mcp

build-all: build build-mcp

integration:
	go test -tags=integration -v -timeout 120s ./test/integration
