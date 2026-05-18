.PHONY: build test lint install clean

build:
	go build -o bin/elevenlabs-pp-cli ./cmd/elevenlabs-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/elevenlabs-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/elevenlabs-pp-mcp ./cmd/elevenlabs-pp-mcp

install-mcp:
	go install ./cmd/elevenlabs-pp-mcp

build-all: build build-mcp
