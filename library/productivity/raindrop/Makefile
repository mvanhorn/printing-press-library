.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	mkdir -p bin
	go build -buildvcs=false -o bin/raindrop$(BIN_EXT) ./cmd/raindrop-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go build -buildvcs=false -o $${GOBIN:-$${HOME}/go/bin}/raindrop$(BIN_EXT) ./cmd/raindrop-pp-cli

clean:
	rm -rf bin/

build-mcp:
	mkdir -p bin
	go build -buildvcs=false -o bin/raindrop-pp-mcp$(BIN_EXT) ./cmd/raindrop-pp-mcp

install-mcp:
	go install ./cmd/raindrop-pp-mcp

build-all: build build-mcp
