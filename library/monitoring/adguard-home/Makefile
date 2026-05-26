.PHONY: build test lint install clean

build:
	go build -o bin/adguard-home-pp-cli ./cmd/adguard-home-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/adguard-home-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/adguard-home-pp-mcp ./cmd/adguard-home-pp-mcp

install-mcp:
	go install ./cmd/adguard-home-pp-mcp

build-all: build build-mcp
