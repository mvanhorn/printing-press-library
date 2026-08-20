.PHONY: build test lint install clean

build:
	go build -o bin/american-reindustrialization-pp-cli ./cmd/american-reindustrialization-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/american-reindustrialization-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/american-reindustrialization-pp-mcp ./cmd/american-reindustrialization-pp-mcp

install-mcp:
	go install ./cmd/american-reindustrialization-pp-mcp

build-all: build build-mcp
