.PHONY: build test lint install clean

build:
	go build -o bin/mobalytics-lol-pp-cli ./cmd/mobalytics-lol-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/mobalytics-lol-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/mobalytics-lol-pp-mcp ./cmd/mobalytics-lol-pp-mcp

install-mcp:
	go install ./cmd/mobalytics-lol-pp-mcp

build-all: build build-mcp
