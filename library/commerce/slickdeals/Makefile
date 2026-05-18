.PHONY: build test lint install clean

build:
	go build -o bin/slickdeals-pp-cli ./cmd/slickdeals-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/slickdeals-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/slickdeals-pp-mcp ./cmd/slickdeals-pp-mcp

install-mcp:
	go install ./cmd/slickdeals-pp-mcp

build-all: build build-mcp
