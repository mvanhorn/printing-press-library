.PHONY: build test lint install clean

build:
	go build -o bin/gravitus-pp-cli ./cmd/gravitus-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/gravitus-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/gravitus-pp-mcp ./cmd/gravitus-pp-mcp

install-mcp:
	go install ./cmd/gravitus-pp-mcp

build-all: build build-mcp
