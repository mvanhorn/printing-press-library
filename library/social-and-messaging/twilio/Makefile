.PHONY: build test lint install clean

build:
	go build -o bin/twilio-pp-cli ./cmd/twilio-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/twilio-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/twilio-pp-mcp ./cmd/twilio-pp-mcp

install-mcp:
	go install ./cmd/twilio-pp-mcp

build-all: build build-mcp
