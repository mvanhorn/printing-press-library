.PHONY: build test lint install clean

build:
	go build -o bin/postscript-pp-cli ./cmd/postscript-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/postscript-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/postscript-pp-mcp ./cmd/postscript-pp-mcp

install-mcp:
	go install ./cmd/postscript-pp-mcp

build-all: build build-mcp
