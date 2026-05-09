.PHONY: build test lint install clean

build:
	go build -o bin/peloton-pp-cli ./cmd/peloton-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/peloton-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/peloton-pp-mcp ./cmd/peloton-pp-mcp

install-mcp:
	go install ./cmd/peloton-pp-mcp

build-all: build build-mcp
