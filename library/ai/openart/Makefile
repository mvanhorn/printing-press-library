.PHONY: build test lint install clean

build:
	go build -o bin/openart-pp-cli ./cmd/openart-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/openart-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/openart-pp-mcp ./cmd/openart-pp-mcp

install-mcp:
	go install ./cmd/openart-pp-mcp

build-all: build build-mcp
