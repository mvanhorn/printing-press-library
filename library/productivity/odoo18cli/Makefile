.PHONY: build test lint install clean

build:
	go build -o bin/odoo18cli-pp-cli ./cmd/odoo18cli-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/odoo18cli-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/odoo18cli-pp-mcp ./cmd/odoo18cli-pp-mcp

install-mcp:
	go install ./cmd/odoo18cli-pp-mcp

build-all: build build-mcp
