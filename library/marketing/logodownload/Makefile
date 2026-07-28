CLI=logodownload-pp-cli
PKG=./cmd/$(CLI)

.PHONY: tidy test vet build install help smoke

tidy:
	go mod tidy

test:
	go test ./...

vet:
	go vet ./...

build:
	go build -o $(CLI) $(PKG)

install:
	go install $(PKG)

help:
	go run $(PKG) --help

smoke:
	go run $(PKG) nike
	go run $(PKG) "Bradesco Logo" --preview --preview-h 6 --preview-w 20 --preview-limit 2
