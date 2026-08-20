.PHONY: build test lint install clean

build:
	go build -o bin/azure-devops-pp-cli ./cmd/azure-devops-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/azure-devops-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/azure-devops-pp-mcp ./cmd/azure-devops-pp-mcp

install-mcp:
	go install ./cmd/azure-devops-pp-mcp

build-all: build build-mcp
