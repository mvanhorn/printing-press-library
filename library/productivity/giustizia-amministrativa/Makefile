.PHONY: build test lint install clean

build:
	go build -o bin/giustizia-amministrativa-pp-cli ./cmd/giustizia-amministrativa-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/giustizia-amministrativa-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/giustizia-amministrativa-pp-mcp ./cmd/giustizia-amministrativa-pp-mcp

install-mcp:
	go install ./cmd/giustizia-amministrativa-pp-mcp

build-all: build build-mcp
