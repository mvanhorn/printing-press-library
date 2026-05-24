.PHONY: build test lint install clean

build:
	go build -o bin/namecheap-pp-cli ./cmd/namecheap-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/namecheap-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/namecheap-pp-mcp ./cmd/namecheap-pp-mcp

install-mcp:
	go install ./cmd/namecheap-pp-mcp

build-all: build build-mcp
