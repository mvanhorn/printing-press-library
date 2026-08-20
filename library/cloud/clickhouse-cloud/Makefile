.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/clickhouse-cloud-pp-cli$(BIN_EXT) ./cmd/clickhouse-cloud-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/clickhouse-cloud-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/clickhouse-cloud-pp-mcp$(BIN_EXT) ./cmd/clickhouse-cloud-pp-mcp

install-mcp:
	go install ./cmd/clickhouse-cloud-pp-mcp

build-all: build build-mcp
