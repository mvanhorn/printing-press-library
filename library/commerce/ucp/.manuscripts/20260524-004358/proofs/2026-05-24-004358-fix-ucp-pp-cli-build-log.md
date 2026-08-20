Using LLM to understand API docs...
warning: claude failed (exit status 1), trying codex
error: unexpected argument '--quiet' found

  tip: to pass '--quiet' as a value, use '-- --quiet'

Usage: codex [OPTIONS] [PROMPT]
       codex [OPTIONS] <COMMAND> [ARGS]

For more information, try '--help'.
warning: LLM doc-to-spec failed, falling back to regex: LLM doc-to-spec failed: codex failed: exit status 2
PASS go mod tidy
PASS govulncheck ./...
PASS go vet ./...
PASS go build ./...
PASS build runnable binary
PASS ucp-pp-cli --help
PASS ucp-pp-cli version
PASS ucp-pp-cli doctor
Generated ucp at /Users/dave/printing-press/.runstate/openbrain-3654f44d/runs/20260524-004358/working/ucp-pp-cli (from docs)
Bundled /Users/dave/printing-press/.runstate/openbrain-3654f44d/runs/20260524-004358/working/ucp-pp-cli/build/ucp-pp-mcp-darwin-arm64.mcpb
