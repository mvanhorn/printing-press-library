.PHONY: build test lint install clean

build:
	go build -o bin/defillama-pp-cli ./cmd/defillama-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/defillama-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/defillama-pp-mcp ./cmd/defillama-pp-mcp

install-mcp:
	go install ./cmd/defillama-pp-mcp

build-all: build build-mcp
