# ☁️ cloud.md — grants-pp-cli (memory-manager, <200 lines)

> Last update: 2026-07-05 · Status: **v1 done, verified against live APIs**

## 📌 Status
- CLI: `grants-pp-cli` — open research grants, keyless (Grants.gov + NIH + NSF)
- Build: ✔ `go vet` clean, `go build` OK, `go test` green (12 subtests)
- Live verification: ✔ doctor reports all 3 sources UP; search/nih/nsf return real rows;
  `--closing-before`, `--min-award`, `--min-amount` demonstrably filter
- Deployment: ⏸ GitHub push/PR **awaiting user approval** (no auto-push)

## 🧱 Layout
```
cmd/grants-pp-cli/main.go       entry point
internal/sources/               http.go, grantsgov.go, nih.go, nsf.go, money.go
internal/cli/                   root, search, nih, nsf, doctor, filter (+filter_test)
```

## 📐 Design rules (retraction-checker pattern)
- Keyless: no API key, no .env, no registration. (security scan: 0 secret literals)
- No `exec.Command` (scan: 0 hits). Every call goes through `net/http` directly.
- Stdlib-only: no go.sum, zero external dependencies. 731 LOC.
- Filter logic in pure functions (filter.go) → unit-tested.

## ⚠️ Known limits
- Grants.gov `awardCeiling` is often 0 → `AwardCap()` falls back to `estimatedFunding`
  (the display marks these with an "estimated" label).
- NSF keyword relevance is loose (full-text OR); `--min-amount` does filter, but the list
  can be broader than expected — that is the API's behaviour, not a bug.
- NIH/NSF return *awarded* grants (benchmark); *open* opportunities come from Grants.gov.

## ▶️ Next
`deployment` agent: `git init` + push + PR — ONLY after the user says "go".
