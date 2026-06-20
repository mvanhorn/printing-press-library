.PHONY: build test lint install clean

build:
	go build -o bin/dbt-cloud-pp-cli ./cmd/dbt-cloud-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/dbt-cloud-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/dbt-cloud-pp-mcp ./cmd/dbt-cloud-pp-mcp

install-mcp:
	go install ./cmd/dbt-cloud-pp-mcp

build-all: build build-mcp
