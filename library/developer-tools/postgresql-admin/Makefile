.PHONY: build test lint install clean

build:
	go build -o bin/postgresql-admin-pp-cli ./cmd/postgresql-admin-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/postgresql-admin-pp-cli

clean:
	rm -rf bin/
