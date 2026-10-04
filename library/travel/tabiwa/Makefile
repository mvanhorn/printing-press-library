.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -o bin/tabiwa-pp-cli$(BIN_EXT) ./cmd/tabiwa-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/tabiwa-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -trimpath -o bin/tabiwa-pp-mcp$(BIN_EXT) ./cmd/tabiwa-pp-mcp

install-mcp:
	go install ./cmd/tabiwa-pp-mcp

build-all: build build-mcp
