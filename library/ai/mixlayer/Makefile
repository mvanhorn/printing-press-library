.PHONY: build test lint install clean

build:
	go build -o bin/mixlayer-pp-cli ./cmd/mixlayer-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/mixlayer-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/mixlayer-pp-mcp ./cmd/mixlayer-pp-mcp

install-mcp:
	go install ./cmd/mixlayer-pp-mcp

build-all: build build-mcp
