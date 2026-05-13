.PHONY: build test lint install clean

build:
	go build -o bin/hubspot-pp-cli ./cmd/hubspot-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/hubspot-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/hubspot-pp-mcp ./cmd/hubspot-pp-mcp

install-mcp:
	go install ./cmd/hubspot-pp-mcp

build-all: build build-mcp
