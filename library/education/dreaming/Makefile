.PHONY: build test lint install clean

build:
	go build -o bin/dreaming-pp-cli ./cmd/dreaming-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/dreaming-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/dreaming-pp-mcp ./cmd/dreaming-pp-mcp

install-mcp:
	go install ./cmd/dreaming-pp-mcp

build-all: build build-mcp
