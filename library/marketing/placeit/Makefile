.PHONY: build test lint install clean

build:
	go build -o bin/placeit-pp-cli ./cmd/placeit-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/placeit-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/placeit-pp-mcp ./cmd/placeit-pp-mcp

install-mcp:
	go install ./cmd/placeit-pp-mcp

build-all: build build-mcp
