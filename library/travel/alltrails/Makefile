.PHONY: build test lint install clean

build:
	go build -o bin/alltrails-pp-cli ./cmd/alltrails-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/alltrails-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/alltrails-pp-mcp ./cmd/alltrails-pp-mcp

install-mcp:
	go install ./cmd/alltrails-pp-mcp

build-all: build build-mcp
