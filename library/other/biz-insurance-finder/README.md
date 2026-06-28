# biz-insurance-finder

A guided CLI assistant that helps a **US small business find and apply for
commercial insurance** — starting with General Liability. It interviews the
business once, saves a reusable applicant profile, then for each matched
provider gives you the quote-start URL, a paste-ready answer sheet, and a
checklist of the manual steps it will **not** do for you.

It guides you through **your own browser**. It never fills a form, solves a
CAPTCHA, enters an EIN/SSN or payment, or submits anything on your behalf.

> The single most important rule it encodes, learned from a real multi-carrier
> quote run: an **importer / private-label / manufacturer ("deemed
> manufacturer") must be routed to specialty markets**, not the mainstream
> instant-quote carriers (Hartford, biBerk, Next) that decline that class.

## Quick Start

```bash
# build (Go 1.26+)
go build -o biz-insurance-finder ./cmd/biz-insurance-finder-pp-cli

# 1) one-time interview -> ./profile.json (gitignored; may contain PII)
./biz-insurance-finder intake

# 2) ranked shortlist of providers, with reasons
./biz-insurance-finder match

# 3) full guided plan: warnings + per-provider URL, answer sheet, checklist
./biz-insurance-finder guide

# inspect a single provider's answer sheet or manual checklist
./biz-insurance-finder answersheet insurance-canopy
./biz-insurance-finder checklist insurance-canopy
```

Commands:

| Command | What it does |
|---------|--------------|
| `intake` | Interview the business; save/update `profile.json` (one question at a time, sensible defaults). |
| `profile [show\|path\|validate]` | Show, locate, or validate the saved profile. |
| `providers [list\|show <id>]` | List/inspect the editable provider registry. |
| `match [--shortlist]` | Rank providers for your profile (importer → specialty markets). |
| `answersheet [<id>] [--all]` | Paste-ready field values for a provider's quote form. |
| `checklist [<id>]` | The manual actions the tool will NOT do (CAPTCHA, account, EIN/SSN, payment, submit). |
| `warnings` | Underwriting landmines for your profile (foreign-products exclusion, Coverage B IP gap, ...). |
| `guide [--top N] [--all]` | End-to-end walkthrough combining all of the above. |
| `doctor` | Health-check the registry, profile, and paths. |

## Output Formats

Every command supports machine-readable output:

```bash
./biz-insurance-finder match --json                 # JSON (also the default when piped)
./biz-insurance-finder match --csv                  # CSV for array/table results
./biz-insurance-finder match --plain                # tab-separated values
./biz-insurance-finder match --json --select tier,score
./biz-insurance-finder match --json --compact       # drop verbose fields for fewer tokens
./biz-insurance-finder providers list --quiet       # suppress output; use the exit code
```

Color is **off by default** (agent-safe). Add `--human-friendly` for color in a
terminal; `NO_COLOR=1` always disables it.

## Agent Usage

Add `--agent` to any command for agent-friendly defaults
(`--json --compact --no-input --no-color --yes`):

```bash
biz-insurance-finder --agent match
biz-insurance-finder --agent --profile ./acme.json guide --top 3
```

`intake` is non-interactive under `--no-input`/`--agent` (every answer takes its
default). Agents can also write `profile.json` directly — it is plain JSON whose
shape is documented in [`internal/insurance/types.go`](internal/insurance/types.go).
Exit codes: `0` ok, `2` usage, `3` not found, `5` data error, `10` config error.

## Editing the provider registry

The registry is **data**, not code — edit it freely. Resolution order:

1. `--providers <path>`
2. `$BIZ_INSURANCE_FINDER_PROVIDERS`
3. `./providers.json` in the working directory
4. the registry embedded in the binary

```bash
cp providers.json my-providers.json   # tweak appetites, add carriers, fix details
biz-insurance-finder --providers my-providers.json match
```

Each provider carries a machine-readable `appetite` (per-class fit ratings) plus
`instant_quote` and `unverified` flags; the matcher reads those, so changing the
data changes the ranking with no recompile. Entries marked `"unverified": true`
(e.g. **Supersure**) are surfaced with a warning and penalized in ranking until
confirmed.

## Troubleshooting

| Symptom | Fix |
|---------|-----|
| `no profile at ...` | Run `biz-insurance-finder intake` first. |
| `no provider with id ...` | Run `biz-insurance-finder providers list` to see valid ids. |
| Color codes in piped output | Color is already off when piped; if forcing `--human-friendly`, drop it or set `NO_COLOR=1`. |
| Registry won't load | Run `biz-insurance-finder doctor`; check your `providers.json` is valid JSON. |
| Profile has PII in git | `profile.json` is gitignored by default — keep it that way. |

## Doctor

```bash
biz-insurance-finder doctor
```

Doctor runs local health checks (no network): the provider registry parses, the
applicant profile loads and validates, and the profile directory is writable.

## Scope & safety

- **The tool never acts in your browser.** Filling forms, CAPTCHAs, account
  creation, EIN/SSN, payment, and the final submit are all human steps,
  surfaced per provider by `checklist`.
- **Two-gate submit:** review the filled values, then *you* click submit.
- **This is not insurance advice.** It organizes your answers and points you at
  markets; confirm coverage terms (especially **no foreign-products exclusion**
  for importers) in writing before you bind.

## Project layout

```
cmd/biz-insurance-finder-pp-cli/   # main entrypoint
internal/insurance/            # pure domain: registry, match, answer sheet, checklist, warnings, profile
internal/cli/                  # cobra commands + output formatting
internal/config/               # path/env resolution
providers.json                 # the editable registry (also embedded in the binary)
```

Run the tests: `go test ./...`
