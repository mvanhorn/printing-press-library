.PHONY: build test lint install clean

build:
	go build -o bin/pangolin-pp-cli ./cmd/pangolin-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/pangolin-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/pangolin-pp-mcp ./cmd/pangolin-pp-mcp

install-mcp:
	go install ./cmd/pangolin-pp-mcp

build-all: build build-mcp
