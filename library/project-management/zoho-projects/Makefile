.PHONY: build test lint install clean

build:
	go build -o bin/zoho-projects-pp-cli ./cmd/zoho-projects-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/zoho-projects-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/zoho-projects-pp-mcp ./cmd/zoho-projects-pp-mcp

install-mcp:
	go install ./cmd/zoho-projects-pp-mcp

build-all: build build-mcp
