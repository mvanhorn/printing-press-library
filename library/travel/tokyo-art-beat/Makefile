.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -o bin/tokyo-art-beat-pp-cli$(BIN_EXT) ./cmd/tokyo-art-beat-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/tokyo-art-beat-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -trimpath -o bin/tokyo-art-beat-pp-mcp$(BIN_EXT) ./cmd/tokyo-art-beat-pp-mcp

install-mcp:
	go install ./cmd/tokyo-art-beat-pp-mcp

build-all: build build-mcp
