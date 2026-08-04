.PHONY: build test lint install clean

build:
	go build -o bin/nisifilters-pp-cli ./cmd/nisifilters-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/nisifilters-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/nisifilters-pp-mcp ./cmd/nisifilters-pp-mcp

install-mcp:
	go install ./cmd/nisifilters-pp-mcp

build-all: build build-mcp
