.PHONY: build test lint install clean

build:
	go build -o bin/render-pp-cli ./cmd/render-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/render-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/render-pp-mcp ./cmd/render-pp-mcp

install-mcp:
	go install ./cmd/render-pp-mcp

build-all: build build-mcp
