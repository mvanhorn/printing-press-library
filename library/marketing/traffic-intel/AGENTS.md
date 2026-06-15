# AGENTS.md — traffic-intel

This is a private conventional Printing Press library module. It is not registered through generated registry files.

## Agent guidance

- Keep the module buildable as a standalone Go module.
- Do not edit generated `registry.json` or `cli-skills` for this private MVP.
- Do not import sibling CLI `internal` packages; use local data or shell-out discovery only.
- Keep tests offline and credential-free.
- Use `--agent` for automation; it enables JSON and non-interactive flags.
- Keep `.printing-press.json` and `agent-context` schema details in sync when commands or source plans change.

## Verification

```bash
go test ./...
go build ./cmd/traffic-intel-pp-cli
make smoke
```
