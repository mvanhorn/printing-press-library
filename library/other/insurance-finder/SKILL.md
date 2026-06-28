---
name: pp-insurance-finder
description: "Guided assistant for finding and applying for US small-business commercial insurance (General Liability first): one-time intake, importer-aware provider matching, paste-ready answer sheets, and per-provider manual-action checklists. Trigger phrases: `find commercial insurance for my business`, `which insurers should I get a general liability quote from`, `help me apply for business liability insurance`, `I import/private-label products, who will insure me`, `fill out my insurance quote answer sheet`, `use insurance-finder-pp-cli`, `run insurance-finder`."
author: "beetz12"
license: "MIT"
argument-hint: "<command> [args] | install cli"
allowed-tools: "Read Bash"
metadata: '{"openclaw":{"requires":{"bins":["insurance-finder-pp-cli"]},"install":[{"id":"go","kind":"shell","command":"go install github.com/mvanhorn/printing-press-library/library/other/insurance-finder/cmd/insurance-finder-pp-cli@latest","bins":["insurance-finder-pp-cli"],"label":"Install via go install"}]}}'
---

# Insurance Finder — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `insurance-finder-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install insurance-finder --cli-only
   ```
2. Verify: `insurance-finder-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/other/insurance-finder/cmd/insurance-finder-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Use it when a US small business needs to find and apply for commercial General Liability insurance and wants a consistent, reusable way to do it:

- Run a one-time interview that saves a reusable applicant profile (`intake`).
- Get a ranked shortlist of insurers/brokers matched to the business class (`match`).
- Produce paste-ready answer sheets per provider so every quote form is filled the same way (`answersheet`, `guide`).
- See the manual steps the tool will not do (CAPTCHA, account, EIN/SSN, payment, submit) (`checklist`).
- Surface underwriting landmines such as the foreign-products exclusion for importers (`warnings`).

It is especially useful for an **importer / private-label / manufacturer** ("deemed manufacturer"), which must be routed to specialty markets rather than the mainstream instant-quote carriers that decline that class.

## When Not to Use This CLI

Do not use it to fill, submit, or pay for anything. This CLI is a read-only guide: it produces answer sheets and checklists and points you at quote-start URLs. It never drives a browser, solves a CAPTCHA, enters a government ID or payment, or submits a quote. It is not insurance, legal, or financial advice.

## Unique Capabilities

- **Importer-aware routing** (`match`) — a data-driven appetite registry routes importers/private-label/manufacturers to specialty markets and pushes mainstream decliners to an "avoid" tier.
- **Per-provider answer sheet** (`answersheet`) — maps the stored profile to exact paste-ready field values, with provider-specific hints.
- **Manual-actions checklist** (`checklist`) — the human-only steps: CAPTCHA, account/password, EIN/SSN, payment, and the two-gate submit approval.
- **Underwriting warnings** (`warnings`) — foreign-products exclusion, GL Coverage B IP gap, lead-capture-at-contact-step, next-business-day start.
- **End-to-end guide** (`guide`) — combines warnings + per-provider URL, answer sheet, and checklist.

## Commands

```bash
insurance-finder-pp-cli intake             # interview -> profile.json (reusable, gitignored PII)
insurance-finder-pp-cli match              # ranked provider shortlist with reasons
insurance-finder-pp-cli guide              # warnings + per-provider URL, answer sheet, checklist
insurance-finder-pp-cli answersheet <id>   # paste-ready field values for one provider
insurance-finder-pp-cli checklist <id>     # manual actions the tool will not do
insurance-finder-pp-cli warnings           # underwriting landmines for the profile
insurance-finder-pp-cli providers list     # the editable provider registry
insurance-finder-pp-cli doctor             # health-check registry + profile + paths
```

## Output Formats

Add `--json`, `--csv`, `--plain`, `--select <fields>`, or `--compact` to any command. Add `--agent` for agent-friendly defaults (`--json --compact --no-input --no-color --yes`). Color is off by default; `--human-friendly` enables it in a terminal.

## Doctor

```bash
insurance-finder-pp-cli doctor
```

Runs local health checks (no network): the provider registry parses, the saved profile loads and validates, and the profile directory is writable.
