.PHONY: build test lint install clean

build:
	go build -o bin/hotel-tonight-pp-cli ./cmd/hotel-tonight-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/hotel-tonight-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/hotel-tonight-pp-mcp ./cmd/hotel-tonight-pp-mcp

install-mcp:
	go install ./cmd/hotel-tonight-pp-mcp

build-all: build build-mcp
