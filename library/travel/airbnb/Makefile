.PHONY: build test lint install clean

build:
	go build -o bin/airbnb-pp-cli ./cmd/airbnb-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/airbnb-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/airbnb-pp-mcp ./cmd/airbnb-pp-mcp

install-mcp:
	go install ./cmd/airbnb-pp-mcp

build-all: build build-mcp
