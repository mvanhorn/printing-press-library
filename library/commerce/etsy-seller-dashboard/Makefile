.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/etsy-seller-dashboard-pp-cli$(BIN_EXT) ./cmd/etsy-seller-dashboard-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/etsy-seller-dashboard-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/etsy-seller-dashboard-pp-mcp$(BIN_EXT) ./cmd/etsy-seller-dashboard-pp-mcp

install-mcp:
	go install ./cmd/etsy-seller-dashboard-pp-mcp

build-all: build build-mcp
