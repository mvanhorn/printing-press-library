.PHONY: build test lint install clean

build:
	go build -o bin/sendgrid-pp-cli ./cmd/sendgrid-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/sendgrid-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/sendgrid-pp-mcp ./cmd/sendgrid-pp-mcp

install-mcp:
	go install ./cmd/sendgrid-pp-mcp

build-all: build build-mcp
