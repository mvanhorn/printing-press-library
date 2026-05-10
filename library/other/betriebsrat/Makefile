.PHONY: build test lint install clean

build:
	go build -o bin/betriebsrat-pp-cli ./cmd/betriebsrat-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/betriebsrat-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/betriebsrat-pp-mcp ./cmd/betriebsrat-pp-mcp

install-mcp:
	go install ./cmd/betriebsrat-pp-mcp

build-all: build build-mcp
