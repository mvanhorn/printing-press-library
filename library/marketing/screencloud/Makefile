.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/screencloud-pp-cli$(BIN_EXT) ./cmd/screencloud-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/screencloud-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/screencloud-pp-mcp$(BIN_EXT) ./cmd/screencloud-pp-mcp

install-mcp:
	go install ./cmd/screencloud-pp-mcp

build-all: build build-mcp
