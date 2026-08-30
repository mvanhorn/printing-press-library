.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -o bin/zotero-research-library-pp-cli$(BIN_EXT) ./cmd/zotero-research-library-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/zotero-research-library-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/zotero-research-library-pp-mcp$(BIN_EXT) ./cmd/zotero-research-library-pp-mcp

install-mcp:
	go install ./cmd/zotero-research-library-pp-mcp

build-all: build build-mcp
