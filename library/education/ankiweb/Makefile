.PHONY: build test lint install clean

build:
	go build -o bin/ankiweb-pp-cli ./cmd/ankiweb-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/ankiweb-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/ankiweb-pp-mcp ./cmd/ankiweb-pp-mcp

install-mcp:
	go install ./cmd/ankiweb-pp-mcp

build-all: build build-mcp
