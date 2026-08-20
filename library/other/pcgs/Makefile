.PHONY: build test lint install clean

build:
	go build -o bin/pcgs-pp-cli ./cmd/pcgs-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/pcgs-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/pcgs-pp-mcp ./cmd/pcgs-pp-mcp

install-mcp:
	go install ./cmd/pcgs-pp-mcp

build-all: build build-mcp
