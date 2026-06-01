.PHONY: build test lint install clean

build:
	go build -o bin/google-cloud-run-pp-cli ./cmd/google-cloud-run-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/google-cloud-run-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/google-cloud-run-pp-mcp ./cmd/google-cloud-run-pp-mcp

install-mcp:
	go install ./cmd/google-cloud-run-pp-mcp

build-all: build build-mcp
