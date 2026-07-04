.PHONY: build test lint install clean

build:
	go build -o bin/isitagentready-pp-cli ./cmd/isitagentready-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/isitagentready-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/isitagentready-pp-mcp ./cmd/isitagentready-pp-mcp

install-mcp:
	go install ./cmd/isitagentready-pp-mcp

build-all: build build-mcp
