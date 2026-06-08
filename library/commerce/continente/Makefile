.PHONY: build test lint install clean vet verify-release validate-publish

build:
	go build -o bin/continente-pp-cli ./cmd/continente-pp-cli

test:
	go test ./...

vet:
	go vet ./...

lint:
	golangci-lint run

install:
	go install ./cmd/continente-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/continente-pp-mcp ./cmd/continente-pp-mcp

install-mcp:
	go install ./cmd/continente-pp-mcp

build-all: build build-mcp

verify-release: test vet build-all

validate-publish:
	cli-printing-press publish validate --dir .
