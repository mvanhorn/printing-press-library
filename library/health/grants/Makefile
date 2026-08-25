.PHONY: build test vet install clean

build:
	go build -o bin/grants-pp-cli ./cmd/grants-pp-cli

test:
	go test ./...

vet:
	go vet ./...

install:
	go install ./cmd/grants-pp-cli

clean:
	rm -rf bin/
