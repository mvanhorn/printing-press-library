.PHONY: build test lint install clean

build:
	go build -o bin/google-photos-pp-cli ./cmd/google-photos-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/google-photos-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/google-photos-pp-mcp ./cmd/google-photos-pp-mcp

install-mcp:
	go install ./cmd/google-photos-pp-mcp

build-all: build build-mcp
