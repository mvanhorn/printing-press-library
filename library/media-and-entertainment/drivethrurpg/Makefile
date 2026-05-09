.PHONY: build test lint install clean

build:
	go build -o bin/drivethrurpg-pp-cli ./cmd/drivethrurpg-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/drivethrurpg-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/drivethrurpg-pp-mcp ./cmd/drivethrurpg-pp-mcp

install-mcp:
	go install ./cmd/drivethrurpg-pp-mcp

build-all: build build-mcp
