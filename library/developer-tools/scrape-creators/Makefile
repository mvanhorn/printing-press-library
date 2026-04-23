.PHONY: build test lint install clean

build:
	go build -o bin/scrape-creators-pp-cli ./cmd/scrape-creators-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/scrape-creators-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/scrape-creators-pp-mcp ./cmd/scrape-creators-pp-mcp

install-mcp:
	go install ./cmd/scrape-creators-pp-mcp

build-all: build build-mcp
