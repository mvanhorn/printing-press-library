.PHONY: build test lint install clean

build:
	go build -o bin/air-quality-pp-cli ./cmd/air-quality-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/air-quality-pp-cli

clean:
	rm -rf bin/
