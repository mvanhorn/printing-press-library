.PHONY: build test lint install clean

build:
	go build -o bin/tidycal-pp-cli ./cmd/tidycal-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/tidycal-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/tidycal-pp-mcp ./cmd/tidycal-pp-mcp

install-mcp:
	go install ./cmd/tidycal-pp-mcp

build-all: build build-mcp
