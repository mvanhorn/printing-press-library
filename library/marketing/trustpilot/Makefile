.PHONY: build test lint install clean

build:
	go build -o bin/trustpilot-pp-cli ./cmd/trustpilot-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/trustpilot-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/trustpilot-pp-mcp ./cmd/trustpilot-pp-mcp

install-mcp:
	go install ./cmd/trustpilot-pp-mcp

build-all: build build-mcp
