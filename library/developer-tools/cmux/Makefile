.PHONY: build test lint install clean

build:
	go build -o bin/cmux-pp-cli ./cmd/cmux-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/cmux-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/cmux-pp-mcp ./cmd/cmux-pp-mcp

install-mcp:
	go install ./cmd/cmux-pp-mcp

build-all: build build-mcp
