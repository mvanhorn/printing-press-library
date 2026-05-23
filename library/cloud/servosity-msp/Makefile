.PHONY: build test lint install clean

build:
	go build -o bin/servosity-msp-cli ./cmd/servosity-msp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/servosity-msp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/servosity-msp-mcp ./cmd/servosity-msp-mcp

install-mcp:
	go install ./cmd/servosity-msp-mcp

build-all: build build-mcp
