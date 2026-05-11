.PHONY: build test lint install clean

build:
	go build -o bin/jira-cloud-platform-pp-cli ./cmd/jira-cloud-platform-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/jira-cloud-platform-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/jira-cloud-platform-pp-mcp ./cmd/jira-cloud-platform-pp-mcp

install-mcp:
	go install ./cmd/jira-cloud-platform-pp-mcp

build-all: build build-mcp
