.PHONY: build test lint install clean

build:
	go build -o bin/marianatek-pp-cli ./cmd/marianatek-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/marianatek-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/marianatek-pp-mcp ./cmd/marianatek-pp-mcp

install-mcp:
	go install ./cmd/marianatek-pp-mcp

build-all: build build-mcp
