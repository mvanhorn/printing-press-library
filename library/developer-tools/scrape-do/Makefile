.PHONY: build test lint install clean

build:
	go build -o bin/scrape-do-pp-cli ./cmd/scrape-do-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/scrape-do-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/scrape-do-pp-mcp ./cmd/scrape-do-pp-mcp

install-mcp:
	go install ./cmd/scrape-do-pp-mcp

build-all: build build-mcp
