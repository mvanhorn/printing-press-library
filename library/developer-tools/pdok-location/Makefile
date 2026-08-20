.PHONY: build test lint install clean

build:
	go build -o bin/pdok-location-pp-cli ./cmd/pdok-location-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/pdok-location-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/pdok-location-pp-mcp ./cmd/pdok-location-pp-mcp

install-mcp:
	go install ./cmd/pdok-location-pp-mcp

build-all: build build-mcp
