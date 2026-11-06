.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -o bin/lacourt-holidays-pp-cli$(BIN_EXT) ./cmd/lacourt-holidays-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/lacourt-holidays-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -trimpath -o bin/lacourt-holidays-pp-mcp$(BIN_EXT) ./cmd/lacourt-holidays-pp-mcp

install-mcp:
	go install ./cmd/lacourt-holidays-pp-mcp

build-all: build build-mcp
