.PHONY: build test lint install clean

build:
	go build -o bin/godaddy-pp-cli ./cmd/godaddy-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/godaddy-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/godaddy-pp-mcp ./cmd/godaddy-pp-mcp

install-mcp:
	go install ./cmd/godaddy-pp-mcp

build-all: build build-mcp
