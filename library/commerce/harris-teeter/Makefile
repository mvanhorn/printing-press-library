.PHONY: build test lint install clean

build:
	go build -o bin/harris-teeter-pp-cli ./cmd/harris-teeter-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/harris-teeter-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/harris-teeter-pp-mcp ./cmd/harris-teeter-pp-mcp

install-mcp:
	go install ./cmd/harris-teeter-pp-mcp

build-all: build build-mcp
