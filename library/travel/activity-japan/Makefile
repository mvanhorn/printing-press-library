.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -o bin/activity-japan-pp-cli$(BIN_EXT) ./cmd/activity-japan-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/activity-japan-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -trimpath -o bin/activity-japan-pp-mcp$(BIN_EXT) ./cmd/activity-japan-pp-mcp

install-mcp:
	go install ./cmd/activity-japan-pp-mcp

build-all: build build-mcp
