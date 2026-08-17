.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/ticketmaster-pp-cli$(BIN_EXT) ./cmd/ticketmaster-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/ticketmaster-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/ticketmaster-pp-mcp$(BIN_EXT) ./cmd/ticketmaster-pp-mcp

install-mcp:
	go install ./cmd/ticketmaster-pp-mcp

build-all: build build-mcp
