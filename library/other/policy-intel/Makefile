.PHONY: build test lint install clean

build:
	go build -o bin/policy-intel-pp-cli ./cmd/policy-intel-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/policy-intel-pp-cli

clean:
	rm -rf bin/
