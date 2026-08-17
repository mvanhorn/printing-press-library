.PHONY: build test lint install clean

build:
	go build -o bin/drug-enforcement-pp-cli ./cmd/drug-enforcement-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/drug-enforcement-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/drug-enforcement-pp-mcp ./cmd/drug-enforcement-pp-mcp

install-mcp:
	go install ./cmd/drug-enforcement-pp-mcp

build-all: build build-mcp
