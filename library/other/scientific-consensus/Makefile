.PHONY: build test lint install clean

build:
	go build -o bin/scientific-consensus-pp-cli ./cmd/scientific-consensus-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/scientific-consensus-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/scientific-consensus-pp-mcp ./cmd/scientific-consensus-pp-mcp

install-mcp:
	go install ./cmd/scientific-consensus-pp-mcp

build-all: build build-mcp
