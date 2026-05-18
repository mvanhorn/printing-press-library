.PHONY: build test lint install clean

build:
	go build -o bin/uspto-tsdr-pp-cli ./cmd/uspto-tsdr-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/uspto-tsdr-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/uspto-tsdr-pp-mcp ./cmd/uspto-tsdr-pp-mcp

install-mcp:
	go install ./cmd/uspto-tsdr-pp-mcp

build-all: build build-mcp
