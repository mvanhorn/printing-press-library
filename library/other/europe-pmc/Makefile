.PHONY: build test lint install clean

build:
	go build -o bin/europe-pmc-pp-cli ./cmd/europe-pmc-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/europe-pmc-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/europe-pmc-pp-mcp ./cmd/europe-pmc-pp-mcp

install-mcp:
	go install ./cmd/europe-pmc-pp-mcp

build-all: build build-mcp
