.PHONY: build test lint install clean

build:
	go build -o bin/sprocket-pp-cli ./cmd/sprocket-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/sprocket-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/sprocket-pp-mcp ./cmd/sprocket-pp-mcp

install-mcp:
	go install ./cmd/sprocket-pp-mcp

build-all: build build-mcp
