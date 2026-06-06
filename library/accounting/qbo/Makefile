.PHONY: tidy vet build test verify

tidy:
	go mod tidy

vet:
	go vet ./...

build:
	go build ./...

test:
	go test ./...

verify: tidy vet build test
