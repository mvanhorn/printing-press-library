# CISA KEV CLI Shipcheck and Live Smoke

Run time: 2026-06-17T10:56:05Z

## Shipcheck

Command:

```bash
cli-printing-press shipcheck --dir library/developer-tools/cisa-kev --spec library/developer-tools/cisa-kev/spec.yaml --research-dir library/developer-tools/cisa-kev/.manuscripts/20260615-150021
```

Result:

- Verdict: PASS
- Legs: 6/6 passed
- Scorecard: 83/100, Grade A
- Runtime verification: 17/17 passed, 0 critical
- Dogfood: PASS
- MCP surface: PASS
- Skill verification: PASS

## Live Smoke

The CISA KEV feed is public and requires no credentials.

| Check | Command | Result |
| --- | --- | --- |
| Doctor | `go run ./cmd/cisa-kev-pp-cli doctor --json --compact` | `config=ok`, `api=reachable`, `auth=not required` |
| Version | `go run ./cmd/cisa-kev-pp-cli version` | `cisa-kev-pp-cli 1.0.0` |
| Agent context | `go run ./cmd/cisa-kev-pp-cli agent-context --json --compact` | schema version 3 command tree emitted |
| Vendor list | `go run ./cmd/cisa-kev-pp-cli vulns list --vendor Adobe --limit 3 --json --compact` | 3 Adobe KEV rows returned |
| Search | `go run ./cmd/cisa-kev-pp-cli vulns search Ivanti --limit 3 --json --compact` | 3 Ivanti KEV rows returned |
| CVE lookup | `go run ./cmd/cisa-kev-pp-cli vulns get CVE-2021-27104 --json --compact` | CVE record returned with parsed CWE values |
| Due-date triage | `go run ./cmd/cisa-kev-pp-cli vulns due --due-before 2026-06-30 --limit 5 --json --compact` | 5 rows returned in due-date order beginning with 2021-11-17 |

## Fix Verification

`vulns due` now filters without the display limit, sorts by `DueDate`, then applies `--limit`. The regression test `TestApplyDueVulnFilterSortsBeforeLimit` covers the prior failure mode where the raw feed order could truncate urgent results before sorting.

Additional checks:

- `go test ./...`: pass
- `go build ./...`: pass
- `go vet ./...`: pass
- `GOTOOLCHAIN=go1.26.4 go run golang.org/x/vuln/cmd/govulncheck@latest ./...`: pass, 0 called vulnerabilities
- `python3 .github/scripts/verify-publish-package/verify_publish_package.py --base-ref origin/main`: pass
- `go run ./tools/generate-registry/main.go --validate cisa-kev`: pass
- `python3 .github/scripts/verify-skill/verify_skill.py --dir library/developer-tools/cisa-kev/`: pass
- `git diff --check`: pass
