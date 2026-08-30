.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/bestfoodtrucks-pp-cli$(BIN_EXT) ./cmd/bestfoodtrucks-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/bestfoodtrucks-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/bestfoodtrucks-pp-mcp$(BIN_EXT) ./cmd/bestfoodtrucks-pp-mcp

install-mcp:
	go install ./cmd/bestfoodtrucks-pp-mcp

build-all: build build-mcp
