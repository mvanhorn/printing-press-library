.PHONY: build test lint install clean

build:
	go build -o bin/safari-history-pp-cli ./cmd/safari-history-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/safari-history-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/safari-history-pp-mcp ./cmd/safari-history-pp-mcp

install-mcp:
	go install ./cmd/safari-history-pp-mcp

build-all: build build-mcp
