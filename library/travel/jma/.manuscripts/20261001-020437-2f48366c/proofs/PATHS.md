# Canonical paths

- Source project: `<source-project>`
- Binary: project `jma-pp-cli`; optional `go build -o jma-cli ./cmd/jma-pp-cli`
- Printing Press staging: `<build-run>/working/jma-pp-cli`
- Local library destination: `<local-library>` (promotion status in FINAL.md)
- Manuscripts: `<archived-run>`
- Receipt ledger: run `pipeline/phase-receipts.jsonl` (run state only, never archived or committed)
- Run-local Go cache: run `go-cache`
- Live captures/measurements/reviews: project `evidence/`

The user project owns edits. Copy it into the staging directory before Press acceptance/promotion. Local promotion is authorized; publishing and PRs are not.
