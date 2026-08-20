.PHONY: build test lint install clean

build:
	go build -o bin/ordertogo-pp-cli ./cmd/ordertogo-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/ordertogo-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/ordertogo-pp-mcp ./cmd/ordertogo-pp-mcp

install-mcp:
	go install ./cmd/ordertogo-pp-mcp

build-all: build build-mcp
