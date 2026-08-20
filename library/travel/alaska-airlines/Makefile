.PHONY: build test lint install clean

build:
	go build -o bin/alaska-airlines-pp-cli ./cmd/alaska-airlines-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/alaska-airlines-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/alaska-airlines-pp-mcp ./cmd/alaska-airlines-pp-mcp

install-mcp:
	go install ./cmd/alaska-airlines-pp-mcp

build-all: build build-mcp
