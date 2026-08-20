.PHONY: build test lint install clean

build:
	go build -o bin/kdpnichefinder-pp-cli ./cmd/kdpnichefinder-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/kdpnichefinder-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/kdpnichefinder-pp-mcp ./cmd/kdpnichefinder-pp-mcp

install-mcp:
	go install ./cmd/kdpnichefinder-pp-mcp

build-all: build build-mcp
