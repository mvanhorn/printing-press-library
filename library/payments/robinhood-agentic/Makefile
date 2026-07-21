.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/robinhood-agentic-pp-cli$(BIN_EXT) ./cmd/robinhood-agentic-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/robinhood-agentic-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/robinhood-agentic-pp-mcp$(BIN_EXT) ./cmd/robinhood-agentic-pp-mcp

install-mcp:
	go install ./cmd/robinhood-agentic-pp-mcp

build-all: build build-mcp
