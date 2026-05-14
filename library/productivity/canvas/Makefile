.PHONY: build test lint install clean

build:
	go build -o bin/canvas-lms-pp-cli ./cmd/canvas-lms-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/canvas-lms-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/canvas-lms-pp-mcp ./cmd/canvas-lms-pp-mcp

install-mcp:
	go install ./cmd/canvas-lms-pp-mcp

build-all: build build-mcp
