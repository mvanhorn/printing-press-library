.PHONY: build test lint install clean

build:
	go build -o bin/miami-dade-clerk-pp-cli ./cmd/miami-dade-clerk-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/miami-dade-clerk-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/miami-dade-clerk-pp-mcp ./cmd/miami-dade-clerk-pp-mcp

install-mcp:
	go install ./cmd/miami-dade-clerk-pp-mcp

build-all: build build-mcp
