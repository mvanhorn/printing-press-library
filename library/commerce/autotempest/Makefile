.PHONY: build test lint install clean

build:
	go build -o bin/autotempest-pp-cli ./cmd/autotempest-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/autotempest-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/autotempest-pp-mcp ./cmd/autotempest-pp-mcp

install-mcp:
	go install ./cmd/autotempest-pp-mcp

build-all: build build-mcp
