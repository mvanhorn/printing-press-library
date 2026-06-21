.PHONY: build test lint install clean

build:
	go build -o bin/outreach-pp-cli ./cmd/outreach-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/outreach-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/outreach-pp-mcp ./cmd/outreach-pp-mcp

install-mcp:
	go install ./cmd/outreach-pp-mcp

build-all: build build-mcp
