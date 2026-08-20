.PHONY: build test lint install clean

build:
	go build -o bin/jobber-pp-cli ./cmd/jobber-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/jobber-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/jobber-pp-mcp ./cmd/jobber-pp-mcp

install-mcp:
	go install ./cmd/jobber-pp-mcp

build-all: build build-mcp
