.PHONY: build test lint install clean

build:
	go build -o bin/open-food-facts-pp-cli ./cmd/open-food-facts-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/open-food-facts-pp-cli

clean:
	rm -rf bin/
