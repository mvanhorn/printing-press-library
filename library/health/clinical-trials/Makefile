.PHONY: build test lint install clean

build:
	go build -o bin/clinical-trials-pp-cli ./cmd/clinical-trials-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/clinical-trials-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/clinical-trials-pp-mcp ./cmd/clinical-trials-pp-mcp

install-mcp:
	go install ./cmd/clinical-trials-pp-mcp

build-all: build build-mcp
