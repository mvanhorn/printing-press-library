.PHONY: build test lint install clean

build:
	go build -o bin/virustotal-pp-cli ./cmd/virustotal-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/virustotal-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/virustotal-pp-mcp ./cmd/virustotal-pp-mcp

install-mcp:
	go install ./cmd/virustotal-pp-mcp

build-all: build build-mcp
