.PHONY: build test lint install clean

build:
	go build -o bin/adminbyrequest-pp-cli ./cmd/adminbyrequest-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/adminbyrequest-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/adminbyrequest-pp-mcp ./cmd/adminbyrequest-pp-mcp

install-mcp:
	go install ./cmd/adminbyrequest-pp-mcp

build-all: build build-mcp
