.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/indmoney-pp-cli$(BIN_EXT) ./cmd/indmoney-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/indmoney-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/indmoney-pp-mcp$(BIN_EXT) ./cmd/indmoney-pp-mcp

install-mcp:
	go install ./cmd/indmoney-pp-mcp

build-all: build build-mcp
