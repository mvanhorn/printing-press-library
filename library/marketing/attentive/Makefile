.PHONY: build test lint install clean

build:
	go build -o bin/attentive-pp-cli ./cmd/attentive-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/attentive-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/attentive-pp-mcp ./cmd/attentive-pp-mcp

install-mcp:
	go install ./cmd/attentive-pp-mcp

build-all: build build-mcp
