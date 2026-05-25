.PHONY: build test lint install clean

build:
	go build -o bin/strava-pp-cli ./cmd/strava-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/strava-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/strava-pp-mcp ./cmd/strava-pp-mcp

install-mcp:
	go install ./cmd/strava-pp-mcp

build-all: build build-mcp
