.PHONY: build test lint install clean

build:
	go build -o bin/nejm-pp-cli ./cmd/nejm-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/nejm-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/nejm-pp-mcp ./cmd/nejm-pp-mcp

install-mcp:
	go install ./cmd/nejm-pp-mcp

build-all: build build-mcp
