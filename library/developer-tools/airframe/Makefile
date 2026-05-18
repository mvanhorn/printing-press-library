.PHONY: build test lint install clean

build:
	go build -o bin/airframe-pp-cli ./cmd/airframe-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/airframe-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/airframe-pp-mcp ./cmd/airframe-pp-mcp

install-mcp:
	go install ./cmd/airframe-pp-mcp

build-all: build build-mcp
