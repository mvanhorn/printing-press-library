.PHONY: build test lint install clean

build:
	go build -o bin/facebook-marketplace-pp-cli ./cmd/facebook-marketplace-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/facebook-marketplace-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/facebook-marketplace-pp-mcp ./cmd/facebook-marketplace-pp-mcp

install-mcp:
	go install ./cmd/facebook-marketplace-pp-mcp

build-all: build build-mcp
