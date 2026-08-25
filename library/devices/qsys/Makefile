.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/qsys-pp-cli$(BIN_EXT) ./cmd/qsys-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/qsys-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/qsys-pp-mcp$(BIN_EXT) ./cmd/qsys-pp-mcp

install-mcp:
	go install ./cmd/qsys-pp-mcp

build-all: build build-mcp
