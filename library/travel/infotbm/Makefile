.PHONY: build test lint install clean

build:
	go build -o bin/infotbm-pp-cli ./cmd/infotbm-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/infotbm-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/infotbm-pp-mcp ./cmd/infotbm-pp-mcp

install-mcp:
	go install ./cmd/infotbm-pp-mcp

build-all: build build-mcp
