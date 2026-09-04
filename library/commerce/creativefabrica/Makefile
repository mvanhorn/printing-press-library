.PHONY: build test lint install clean

build:
	go build -o bin/creativefabrica-pp-cli ./cmd/creativefabrica-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/creativefabrica-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/creativefabrica-pp-mcp ./cmd/creativefabrica-pp-mcp

install-mcp:
	go install ./cmd/creativefabrica-pp-mcp

build-all: build build-mcp
