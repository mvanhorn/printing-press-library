.PHONY: build test lint install clean

build:
	go build -o bin/apptweak-pp-cli ./cmd/apptweak-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/apptweak-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/apptweak-pp-mcp ./cmd/apptweak-pp-mcp

install-mcp:
	go install ./cmd/apptweak-pp-mcp

build-all: build build-mcp
