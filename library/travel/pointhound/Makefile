.PHONY: build test lint install clean

build:
	go build -o bin/pointhound-pp-cli ./cmd/pointhound-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/pointhound-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/pointhound-pp-mcp ./cmd/pointhound-pp-mcp

install-mcp:
	go install ./cmd/pointhound-pp-mcp

build-all: build build-mcp
