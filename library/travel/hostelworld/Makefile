.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -o bin/hostelworld-pp-cli$(BIN_EXT) ./cmd/hostelworld-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/hostelworld-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -trimpath -o bin/hostelworld-pp-mcp$(BIN_EXT) ./cmd/hostelworld-pp-mcp

install-mcp:
	go install ./cmd/hostelworld-pp-mcp

build-all: build build-mcp
