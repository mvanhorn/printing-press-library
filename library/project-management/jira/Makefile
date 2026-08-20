.PHONY: build test lint install clean

build:
	go build -o bin/jira-pp-cli ./cmd/jira-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/jira-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/jira-pp-mcp ./cmd/jira-pp-mcp

install-mcp:
	go install ./cmd/jira-pp-mcp

build-all: build build-mcp
