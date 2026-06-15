.PHONY: build test lint install clean

build:
	go build -o bin/fred-pp-cli ./cmd/fred-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/fred-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/fred-pp-mcp ./cmd/fred-pp-mcp

install-mcp:
	go install ./cmd/fred-pp-mcp

build-all: build build-mcp
