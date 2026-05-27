.PHONY: build test lint install clean

build:
	go build -o bin/eventbrite-pp-cli ./cmd/eventbrite-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/eventbrite-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/eventbrite-pp-mcp ./cmd/eventbrite-pp-mcp

install-mcp:
	go install ./cmd/eventbrite-pp-mcp

build-all: build build-mcp
