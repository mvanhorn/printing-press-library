.PHONY: build test lint install clean

build:
	go build -o bin/linq-pp-cli ./cmd/linq-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/linq-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/linq-pp-mcp ./cmd/linq-pp-mcp

install-mcp:
	go install ./cmd/linq-pp-mcp

build-all: build build-mcp
