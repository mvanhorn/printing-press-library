.PHONY: build test lint install clean

build:
	go build -o bin/bandsintown-pp-cli ./cmd/bandsintown-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/bandsintown-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/bandsintown-pp-mcp ./cmd/bandsintown-pp-mcp

install-mcp:
	go install ./cmd/bandsintown-pp-mcp

build-all: build build-mcp
