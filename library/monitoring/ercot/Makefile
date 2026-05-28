.PHONY: build test lint install clean

build:
	go build -o bin/ercot-pp-cli ./cmd/ercot-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/ercot-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/ercot-pp-mcp ./cmd/ercot-pp-mcp

install-mcp:
	go install ./cmd/ercot-pp-mcp

build-all: build build-mcp
