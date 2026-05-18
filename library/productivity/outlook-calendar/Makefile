.PHONY: build test lint install clean

build:
	go build -o bin/outlook-calendar-pp-cli ./cmd/outlook-calendar-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/outlook-calendar-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/outlook-calendar-pp-mcp ./cmd/outlook-calendar-pp-mcp

install-mcp:
	go install ./cmd/outlook-calendar-pp-mcp

build-all: build build-mcp
