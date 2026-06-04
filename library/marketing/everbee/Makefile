.PHONY: all build test lint install clean build-mcp install-mcp build-all

all: build-all

build:
	go build -o bin/everbee-pp-cli ./cmd/everbee-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/everbee-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/everbee-pp-mcp ./cmd/everbee-pp-mcp

install-mcp:
	go install ./cmd/everbee-pp-mcp

build-all: build build-mcp
