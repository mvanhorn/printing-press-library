.PHONY: build test lint install clean

build:
	go build -o bin/homeassistant-pp-cli ./cmd/homeassistant-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/homeassistant-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/homeassistant-pp-mcp ./cmd/homeassistant-pp-mcp

install-mcp:
	go install ./cmd/homeassistant-pp-mcp

build-all: build build-mcp
