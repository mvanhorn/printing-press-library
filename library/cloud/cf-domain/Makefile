.PHONY: build test lint install clean

build:
	go build -o bin/cf-domain-pp-cli ./cmd/cf-domain-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/cf-domain-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/cf-domain-pp-mcp ./cmd/cf-domain-pp-mcp

install-mcp:
	go install ./cmd/cf-domain-pp-mcp

build-all: build build-mcp
