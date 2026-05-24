.PHONY: build test lint install clean

build:
	go build -o bin/google-calendar-pp-cli ./cmd/google-calendar-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/google-calendar-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/google-calendar-pp-mcp ./cmd/google-calendar-pp-mcp

install-mcp:
	go install ./cmd/google-calendar-pp-mcp

build-all: build build-mcp
