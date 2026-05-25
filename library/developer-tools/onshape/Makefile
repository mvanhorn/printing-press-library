.PHONY: build test lint install clean

build:
	go build -o bin/onshape-pp-cli ./cmd/onshape-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/onshape-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/onshape-pp-mcp ./cmd/onshape-pp-mcp

install-mcp:
	go install ./cmd/onshape-pp-mcp

build-all: build build-mcp
