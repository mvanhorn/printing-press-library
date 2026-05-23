.PHONY: build test lint install clean pdfgen

build:
	go build -o bin/partitaiva24-pp-cli ./cmd/partitaiva24-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/partitaiva24-pp-cli

clean:
	rm -rf bin/

pdfgen:
	@if [ ! -f internal/pdfgen/assets/bundle.js ]; then \
		curl -s -o internal/pdfgen/assets/bundle.js "https://partitaiva24.cloud/assets/themes/p24/dist/assets/js/vue-app-init.bundle.js?ver=2.9.1408"; \
	fi
	cd internal/pdfgen && npm install
	node internal/pdfgen/scripts/patch-bundle.js

build-mcp:
	go build -o bin/partitaiva24-pp-mcp ./cmd/partitaiva24-pp-mcp

install-mcp:
	go install ./cmd/partitaiva24-pp-mcp

build-all: build build-mcp
