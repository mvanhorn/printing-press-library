.PHONY: build test lint install clean

build:
	go build -o bin/supabase-pp-cli ./cmd/supabase-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/supabase-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/supabase-pp-mcp ./cmd/supabase-pp-mcp

install-mcp:
	go install ./cmd/supabase-pp-mcp

build-all: build build-mcp
