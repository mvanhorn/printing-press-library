.PHONY: build test lint install clean

build:
	go build -o bin/servicetitan-salestech-pp-cli ./cmd/servicetitan-salestech-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/servicetitan-salestech-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/servicetitan-salestech-pp-mcp ./cmd/servicetitan-salestech-pp-mcp

install-mcp:
	go install ./cmd/servicetitan-salestech-pp-mcp

build-all: build build-mcp
