.PHONY: build test lint install clean

build:
	go build -o bin/pubsec-tech-pp-cli ./cmd/pubsec-tech-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/pubsec-tech-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/pubsec-tech-pp-mcp ./cmd/pubsec-tech-pp-mcp

install-mcp:
	go install ./cmd/pubsec-tech-pp-mcp

build-all: build build-mcp
