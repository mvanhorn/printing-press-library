.PHONY: build test lint install clean

build:
	go build -o bin/function-health-pp-cli ./cmd/function-health-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/function-health-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/function-health-pp-mcp ./cmd/function-health-pp-mcp

install-mcp:
	go install ./cmd/function-health-pp-mcp

build-all: build build-mcp
