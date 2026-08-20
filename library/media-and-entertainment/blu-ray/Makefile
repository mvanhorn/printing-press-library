.PHONY: build test lint install clean

build:
	go build -o bin/blu-ray-pp-cli ./cmd/blu-ray-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/blu-ray-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/blu-ray-pp-mcp ./cmd/blu-ray-pp-mcp

install-mcp:
	go install ./cmd/blu-ray-pp-mcp

build-all: build build-mcp
