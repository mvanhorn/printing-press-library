.PHONY: build test lint install clean

build:
	go build -o bin/noon-ae-pp-cli ./cmd/noon-ae-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/noon-ae-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/noon-ae-pp-mcp ./cmd/noon-ae-pp-mcp

install-mcp:
	go install ./cmd/noon-ae-pp-mcp

build-all: build build-mcp
