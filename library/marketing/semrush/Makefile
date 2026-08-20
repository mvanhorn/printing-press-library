.PHONY: build test lint install clean

build:
	go build -o bin/semrush-pp-cli ./cmd/semrush-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/semrush-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/semrush-pp-mcp ./cmd/semrush-pp-mcp

install-mcp:
	go install ./cmd/semrush-pp-mcp

build-all: build build-mcp
