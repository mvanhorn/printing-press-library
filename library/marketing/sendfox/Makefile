.PHONY: build test lint install clean

build:
	go build -o bin/sendfox-pp-cli ./cmd/sendfox-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/sendfox-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/sendfox-pp-mcp ./cmd/sendfox-pp-mcp

install-mcp:
	go install ./cmd/sendfox-pp-mcp

build-all: build build-mcp
