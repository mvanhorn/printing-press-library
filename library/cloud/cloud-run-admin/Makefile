.PHONY: build test lint install clean

build:
	go build -o bin/cloud-run-admin-pp-cli ./cmd/cloud-run-admin-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/cloud-run-admin-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/cloud-run-admin-pp-mcp ./cmd/cloud-run-admin-pp-mcp

install-mcp:
	go install ./cmd/cloud-run-admin-pp-mcp

build-all: build build-mcp
