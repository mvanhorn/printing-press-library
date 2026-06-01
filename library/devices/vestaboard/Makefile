.PHONY: build test lint install clean

build:
	go build -o bin/vestaboard-pp-cli ./cmd/vestaboard-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/vestaboard-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/vestaboard-pp-mcp ./cmd/vestaboard-pp-mcp

install-mcp:
	go install ./cmd/vestaboard-pp-mcp

build-all: build build-mcp
