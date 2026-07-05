.PHONY: build test lint install clean

build:
	go build -o bin/airbnb-outreach-pp-cli ./cmd/airbnb-outreach-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/airbnb-outreach-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/airbnb-outreach-pp-mcp ./cmd/airbnb-outreach-pp-mcp

install-mcp:
	go install ./cmd/airbnb-outreach-pp-mcp

build-all: build build-mcp
