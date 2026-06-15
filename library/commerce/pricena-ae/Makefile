.PHONY: build test lint install clean

build:
	go build -o bin/pricena-ae-pp-cli ./cmd/pricena-ae-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/pricena-ae-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/pricena-ae-pp-mcp ./cmd/pricena-ae-pp-mcp

install-mcp:
	go install ./cmd/pricena-ae-pp-mcp

build-all: build build-mcp
