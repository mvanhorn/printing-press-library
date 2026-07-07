.PHONY: build test lint install clean

build:
	go build -o bin/shelters-pp-cli ./cmd/shelters-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/shelters-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/shelters-pp-mcp ./cmd/shelters-pp-mcp

install-mcp:
	go install ./cmd/shelters-pp-mcp

build-all: build build-mcp
