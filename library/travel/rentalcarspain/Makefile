.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/rentalcarspain-pp-cli$(BIN_EXT) ./cmd/rentalcarspain-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/rentalcarspain-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/rentalcarspain-pp-mcp$(BIN_EXT) ./cmd/rentalcarspain-pp-mcp

install-mcp:
	go install ./cmd/rentalcarspain-pp-mcp

build-all: build build-mcp
