.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/mercadolivre-pp-cli$(BIN_EXT) ./cmd/mercadolivre-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/mercadolivre-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/mercadolivre-pp-mcp$(BIN_EXT) ./cmd/mercadolivre-pp-mcp

install-mcp:
	go install ./cmd/mercadolivre-pp-mcp

build-all: build build-mcp
