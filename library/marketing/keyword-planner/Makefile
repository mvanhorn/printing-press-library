.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/keyword-planner-pp-cli$(BIN_EXT) ./cmd/keyword-planner-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/keyword-planner-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/keyword-planner-pp-mcp$(BIN_EXT) ./cmd/keyword-planner-pp-mcp

install-mcp:
	go install ./cmd/keyword-planner-pp-mcp

build-all: build build-mcp
