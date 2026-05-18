.PHONY: build test lint install clean

build:
	go build -o bin/squarespace-pp-cli ./cmd/squarespace-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/squarespace-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/squarespace-pp-mcp ./cmd/squarespace-pp-mcp

install-mcp:
	go install ./cmd/squarespace-pp-mcp

build-all: build build-mcp
