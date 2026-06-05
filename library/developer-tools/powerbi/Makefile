.PHONY: build test lint install clean

build:
	go build -o bin/powerbi-pp-cli ./cmd/powerbi-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/powerbi-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/powerbi-pp-mcp ./cmd/powerbi-pp-mcp

install-mcp:
	go install ./cmd/powerbi-pp-mcp

build-all: build build-mcp
