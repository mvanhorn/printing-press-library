.PHONY: build test lint install clean

build:
	go build -o bin/superhuman-pp-cli ./cmd/superhuman-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/superhuman-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/superhuman-pp-mcp ./cmd/superhuman-pp-mcp

install-mcp:
	go install ./cmd/superhuman-pp-mcp

build-all: build build-mcp
