.PHONY: build test lint install clean

build:
	go build -o bin/sigma-computing-pp-cli ./cmd/sigma-computing-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/sigma-computing-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/sigma-computing-pp-mcp ./cmd/sigma-computing-pp-mcp

install-mcp:
	go install ./cmd/sigma-computing-pp-mcp

build-all: build build-mcp
