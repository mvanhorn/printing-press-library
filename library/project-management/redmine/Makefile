.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/redmine-pp-cli$(BIN_EXT) ./cmd/redmine-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/redmine-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/redmine-pp-mcp$(BIN_EXT) ./cmd/redmine-pp-mcp

install-mcp:
	go install ./cmd/redmine-pp-mcp

build-all: build build-mcp
