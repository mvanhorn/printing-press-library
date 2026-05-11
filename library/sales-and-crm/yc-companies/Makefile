.PHONY: build test lint install clean

build:
	go build -o bin/yc-companies-pp-cli ./cmd/yc-companies-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/yc-companies-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/yc-companies-pp-mcp ./cmd/yc-companies-pp-mcp

install-mcp:
	go install ./cmd/yc-companies-pp-mcp

build-all: build build-mcp
