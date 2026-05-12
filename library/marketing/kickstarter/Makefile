.PHONY: build test lint install clean

build:
	go build -o bin/kickstarter-pp-cli ./cmd/kickstarter-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/kickstarter-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/kickstarter-pp-mcp ./cmd/kickstarter-pp-mcp

install-mcp:
	go install ./cmd/kickstarter-pp-mcp

build-all: build build-mcp
