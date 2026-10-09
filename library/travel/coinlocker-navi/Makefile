.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -o bin/coinlocker-navi-pp-cli$(BIN_EXT) ./cmd/coinlocker-navi-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/coinlocker-navi-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -trimpath -o bin/coinlocker-navi-pp-mcp$(BIN_EXT) ./cmd/coinlocker-navi-pp-mcp

install-mcp:
	go install ./cmd/coinlocker-navi-pp-mcp

build-all: build build-mcp
