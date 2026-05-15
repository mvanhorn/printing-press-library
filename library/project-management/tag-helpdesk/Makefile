.PHONY: build test lint install clean

build:
	go build -o bin/tag-helpdesk-pp-cli ./cmd/tag-helpdesk-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/tag-helpdesk-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/tag-helpdesk-pp-mcp ./cmd/tag-helpdesk-pp-mcp

install-mcp:
	go install ./cmd/tag-helpdesk-pp-mcp

build-all: build build-mcp
