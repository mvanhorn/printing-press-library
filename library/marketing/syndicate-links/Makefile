.PHONY: build test lint install clean

# CLI-only build. The press-generated MCP server was intentionally dropped —
# Syndicate Links ships a hand-tuned MCP server (`syndicate-links-mcp` on npm,
# https://www.npmjs.com/package/syndicate-links-mcp) that we use for the MCP rail.
# See README.md "Why no MCP here?" for the rationale.

build:
	go build -o bin/syndicate-links-pp-cli ./cmd/syndicate-links-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/syndicate-links-pp-cli

clean:
	rm -rf bin/
