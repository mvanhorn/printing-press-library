.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/groq-pp-cli$(BIN_EXT) ./cmd/groq-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/groq-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/groq-pp-mcp$(BIN_EXT) ./cmd/groq-pp-mcp

install-mcp:
	go install ./cmd/groq-pp-mcp

build-all: build build-mcp
