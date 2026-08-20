Manifest transcendence rows: 8 planned, 8 built. Phase 3 will not pass until all 8 ship.

# Figma CLI Phase 3 build log (reprint v4.2.0 -> v4.24.0)

Ported all 8 hand-authored novel commands from the published library into the freshly generated v4.24.0 tree, renamed to the generator's newNovel* wiring convention. Generator stub scaffolds replaced with real implementations.

## Built (8/8 transcendence)
- frame extract, dev-mode dump, comments-audit, orphans, tokens diff, fingerprint, webhooks test, variables explain

## Generator fixes applied during port
- API drift: client.Get gained a ctx first arg in v4.24.0; added cmd.Context() to all c.Get calls (dev_mode.go, frame.go, webhooks_test_cmd.go).
- Generator bug (retro candidate): novel command "webhooks test" was scaffolded into webhooks_test.go — Go excludes _test.go from the build, so newNovelWebhooksTestCmd was undefined. Renamed to webhooks_test_cmd.go.
- Generator collision (retro candidate): generic framework "orphans" (pm_orphans.go, newOrphansCmd) collided with the novel Figma "orphans" (newNovelOrphansCmd), both Use:"orphans" at root. Removed the generic; kept the novel.

## Verification
- go build ./... OK; go vet OK; go test ./internal/cli/... OK; binary 19MB.
