.PHONY: build test lint install clean

build:
	go build -o bin/excalidraw-mcp-pp-cli ./cmd/excalidraw-mcp-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/excalidraw-mcp-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/excalidraw-mcp-pp-mcp ./cmd/excalidraw-mcp-pp-mcp

install-mcp:
	go install ./cmd/excalidraw-mcp-pp-mcp

build-all: build build-mcp
