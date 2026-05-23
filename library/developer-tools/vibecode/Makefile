.PHONY: build test lint install clean

build:
	go build -o bin/vibecode-pp-cli ./cmd/vibecode-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/vibecode-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/vibecode-pp-mcp ./cmd/vibecode-pp-mcp

install-mcp:
	go install ./cmd/vibecode-pp-mcp

build-all: build build-mcp
