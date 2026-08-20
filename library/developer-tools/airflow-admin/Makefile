.PHONY: build test lint install clean

build:
	go build -o bin/airflow-admin-pp-cli ./cmd/airflow-admin-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/airflow-admin-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/airflow-admin-pp-mcp ./cmd/airflow-admin-pp-mcp

install-mcp:
	go install ./cmd/airflow-admin-pp-mcp

build-all: build build-mcp
