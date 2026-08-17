.PHONY: build test lint install clean

build:
	go build -o bin/wahoo-pp-cli ./cmd/wahoo-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/wahoo-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/wahoo-pp-mcp ./cmd/wahoo-pp-mcp

install-mcp:
	go install ./cmd/wahoo-pp-mcp

build-all: build build-mcp
