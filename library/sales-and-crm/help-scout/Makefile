.PHONY: build test lint install clean

build:
	go build -o bin/help-scout-pp-cli ./cmd/help-scout-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/help-scout-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/help-scout-pp-mcp ./cmd/help-scout-pp-mcp

install-mcp:
	go install ./cmd/help-scout-pp-mcp

build-all: build build-mcp
