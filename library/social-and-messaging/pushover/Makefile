.PHONY: build test lint install clean

build:
	go build -o bin/pushover-pp-cli ./cmd/pushover-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/pushover-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/pushover-pp-mcp ./cmd/pushover-pp-mcp

install-mcp:
	go install ./cmd/pushover-pp-mcp

build-all: build build-mcp
