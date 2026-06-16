# AGENTS.md - amazon-operator-intel

This is a private conventional Printing Press library module. It is not registered through generated registry files.

## Agent Guidance

- Keep the module buildable as a standalone Go module.
- Do not edit generated `registry.json` or `cli-skills` for this private CLI.
- Do not import sibling CLI `internal` packages; use local data or shell-out child CLI discovery only.
- Keep tests offline and credential-free.
- Use `--agent` for automation; it enables JSON and non-interactive flags.
- Preserve source evidence when adding fields or parsers.
- Keep `.printing-press.json`, README, SKILL, and `agent-context` command descriptors in sync when commands or source plans change.

## Verification

```bash
gofmt -w ./cmd ./internal
go vet ./...
go test ./...
go build ./cmd/amazon-operator-intel-pp-cli
```
