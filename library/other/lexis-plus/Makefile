.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -o bin/lexis-plus-pp-cli$(BIN_EXT) ./cmd/lexis-plus-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/lexis-plus-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -trimpath -o bin/lexis-plus-pp-mcp$(BIN_EXT) ./cmd/lexis-plus-pp-mcp

install-mcp:
	go install ./cmd/lexis-plus-pp-mcp

build-all: build build-mcp
