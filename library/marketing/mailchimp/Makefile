.PHONY: build test lint install clean

build:
	go build -o bin/mailchimp-pp-cli ./cmd/mailchimp-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/mailchimp-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/mailchimp-pp-mcp ./cmd/mailchimp-pp-mcp

install-mcp:
	go install ./cmd/mailchimp-pp-mcp

build-all: build build-mcp
