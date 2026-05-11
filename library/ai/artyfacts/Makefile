.PHONY: build test lint install clean

build:
	go build -o bin/artyfacts-pp-cli ./cmd/artyfacts-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/artyfacts-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/artyfacts-pp-mcp ./cmd/artyfacts-pp-mcp

install-mcp:
	go install ./cmd/artyfacts-pp-mcp

build-all: build build-mcp
