.PHONY: build test lint install clean

build:
	go build -o bin/postman-explore-pp-cli ./cmd/postman-explore-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/postman-explore-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/postman-explore-pp-mcp ./cmd/postman-explore-pp-mcp

install-mcp:
	go install ./cmd/postman-explore-pp-mcp

build-all: build build-mcp
