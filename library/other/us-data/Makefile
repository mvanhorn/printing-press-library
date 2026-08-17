.PHONY: build test lint install clean

build:
	go build -o bin/us-data-pp-cli ./cmd/us-data-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/us-data-pp-cli

clean:
	rm -rf bin/
