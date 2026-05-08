## Summary

- 

## Published CLI Metadata

Use this section for Printing Press library publishes. Delete it only when the
PR does not add or modify `library/<category>/<slug>/`.

**API:** `<slug>` | **Category:** `<category>` | **Press version:** `<version>`
**Spec:** `<spec_url or spec.yaml>`  
**Required env:** `<none or env vars>`

## What Changed

- 

## CLI Shape

```bash
$ <cli-name> --help
<paste high-signal help output>
```

## What This CLI Does

<Use the first 2-3 README paragraphs or a concise equivalent.>

## Manuscripts

- [Research Brief](./library/<category>/<slug>/.manuscripts/<run-id>/research/)
- [Shipcheck Results](./library/<category>/<slug>/.manuscripts/<run-id>/proofs/)

## Validation

- [ ] `python3 .github/scripts/verify-skill/verify_skill.py --dir library/<category>/<slug>/`
- [ ] `go build ./...` from the touched CLI root
- [ ] `go vet ./...` from the touched CLI root
- [ ] `go test ./...` from the touched CLI root
- [ ] `govulncheck ./...` from the touched CLI root
- [ ] `go run ./tools/generate-skills/main.go` when `library/**/SKILL.md` changed
- [ ] `git diff --check`

## Gaps / Follow-Up

- 
