# Loops Printing Press CLI

Work with Loops from scripts and agents: inspect contacts and audiences, prepare campaigns and workflows, and review events or transactional email sends before execution. This Printing Press CLI covers all 68 operations in the official Loops OpenAPI 1.22.2 specification and adds three agent checks: `team verify`, `audit lifecycle`, and `campaigns preflight`. Every API mutation starts with a redacted preview. No account data or credentials are included in this package.

Created by [@cathrynlavery](https://github.com/cathrynlavery) (Cathryn Lavery).

## Why this CLI

The [official Loops CLI](https://loops.so/docs/cli) already covers contacts, lists, segments, campaigns, events, transactional email, workflows, authentication profiles, and JSON output. This print adds an exact live team check before every write, a redacted preview by default, explicit intent flags, a private duplicate-send journal, and compound lifecycle and campaign checks. The full [command effect matrix](COMMANDS.md) lists every runnable command and its read, change, or send effect.

The Loops API cannot bulk-list contacts. Product activation and usage cohorts must come from a separate approved data source. PostHog's printed CLI can produce behavior cohorts; keep its credential and tenant selection separate from Loops. A useful agent flow is: read a PostHog cohort, inspect Loops contact or audience state with this CLI, review a campaign preflight, then request explicit approval before any send. A first-use or unused-key flow needs the corresponding product events to exist in PostHog first. See [agent workflow opportunities](WORKFLOWS.md) for feasible compound commands and API limits.

## Install

After the catalog entry is published, install the CLI and its agent skill with the Printing Press installer:

```sh
npx -y @mvanhorn/printing-press-library install loops
```

For CLI only, use `npx -y @mvanhorn/printing-press-library install loops --cli-only`. From a source checkout, `go run ./cmd/loops-pp-cli` works without installation. Verify the installed binary safely:

```sh
loops-pp-cli --version
loops-pp-cli doctor --dry-run --json
loops-pp-cli --help
```

`doctor --dry-run` is offline and sends no email. A normal `doctor` reads local configuration and probes API reachability; it never sends email. Live Loops operations need `LOOPS_API_KEY` or the CLI's private `auth set-token` storage. Never put a key in source, a fixture, a command argument, or a log. For multiple teams, select the intended key or client profile and pass the exact expected API team name with `--team` before a write. A mismatch blocks the write.
Read-only account checks also accept `LOOPS_EXPECTED_TEAM` in the environment, which helps noninteractive agents avoid placing a private team name in command arguments. Writes still require an explicit `--team` flag.

## Agent reads

All examples use synthetic values. Read commands are noninteractive with `--agent`, which enables compact JSON. `contacts find --agent` omits identity fields by default; use `--select` only when more fields are necessary.
For commands with private account or contact input, add `--no-learn --no-cache` so the generated local learning loop and response cache do not retain that input.

```sh
loops-pp-cli team verify --team 'Synthetic Team' --agent
loops-pp-cli audit lifecycle --team 'Synthetic Team' --agent
loops-pp-cli campaigns preflight --id SYNTHETIC_CAMPAIGN_ID --team 'Synthetic Team' --agent
loops-pp-cli campaigns preflight --latest-draft --team 'Synthetic Team' --agent
loops-pp-cli contacts find --email person@example.test --agent --data-source live --no-cache
```

`audit lifecycle` reports counts of lists, segments, campaigns, drafts, workflows, event patterns, and transactional templates without fetching contacts. `campaigns preflight` checks the API-visible draft status, audience target, message, and Guardian findings. It cannot establish consent, approve copy, or authorize publication.

The raw API commands provide broader access to lists, segments, campaign metrics, email messages, themes, components, workflow graphs, suppression status, transactional templates, and uploads. Use `--help` on the specific command before providing input. `--select` limits returned fields; `--no-cache` avoids retaining a read response locally.

## Changes and sends

Every API mutation previews only the method, endpoint template, effect, and provided flag names. It makes **no API call** until `--execute` is supplied. `--execute` prints the same redacted preview to stderr, then verifies the selected team live. `--agent` and `--yes` do not bypass these gates.

| Action | Required intent |
| --- | --- |
| Create or update data | `--execute --team 'Expected Team'` |
| Event or transactional send | Add `--confirm-send --idempotency-key UNIQUE_KEY` |
| Email message preview send | Add `--confirm-send` |
| Campaign create/update or transactional publication | Add `--confirm-publish` |
| Suppression removal | Add `--confirm-suppression-removal` |
| Contact or workflow delete | Add `--confirm-destructive` |

The following is an offline preview; the synthetic address is never sent:

```sh
printf '%s' '{"eventName":"synthetic_event","email":"person@example.test"}' | \
  loops-pp-cli events --stdin --idempotency-key SYNTHETIC_KEY --agent
```

For a reviewed live send, add `--execute --team 'Expected Team' --confirm-send` to the same command. Transactional sends use `transactional send-email --stdin` with a JSON object containing the recipient and published template identifier. Pass private payloads through stdin or a private input file so they do not appear in process arguments.

The event and transactional endpoints use Loops' `Idempotency-Key`. A mode-0700 local journal stores only hashes and timestamps and blocks a repeated team/key attempt. Different credential fingerprints have separate claims, even when team display names match. The CLI never automatically repeats a send after an uncertain network result or HTTP 5xx. A definite HTTP 429 releases the claim so a caller can retry later with the **same** key. Check Loops before manually retrying any other uncertain result. Email preview sends have a local duplicate guard because that endpoint does not document the idempotency header. The default rate is 0.9 requests per second, below Loops' stricter content limit; pagination has a 100-page cap for agent audits.

The generated MCP server exposes read tools. Its shared HTTP client rejects Loops writes without the CLI's explicit mutation intent, so use the CLI for reviewed changes and sends.

Bulk `import` previews by default and can write non-send resources with `--execute --team`. It refuses event and transactional bulk sends because each send needs its own key. `workflow archive`, `sync`, and `export` read Loops data and write private local files. Sync, export, and tail exclude contacts because Loops has no bulk contact listing; use `contacts find` with an explicit identifier. See [COMMANDS.md](COMMANDS.md) for command effects.

## Verification status

Go tests use synthetic data and mock HTTP servers. They cover credential handling, team selection, JSON previews and redaction, pagination, rate limiting, duplicate sends, confirmation, uncertain results, and API errors. Read-only team verification and lifecycle auditing have also passed against a live account without recording account details. A controlled send remains unverified and requires separate explicit approval.

Sources: [Loops API introduction](https://loops.so/docs/api-reference/intro), [official OpenAPI](https://app.loops.so/openapi.json), [official CLI](https://loops.so/docs/cli).

## Quick Start

```bash
# Confirm the CLI version.
loops-pp-cli version

# Check the doctor path without making an API call.
loops-pp-cli doctor --dry-run --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.
- **`team verify`** — Verify the selected API team before a change, preview writes without private values, and journal send attempts.

  ```bash
  loops-pp-cli team verify --agent
  ```
- **`audit lifecycle`** — Count communication resources without retrieving contact records.

  ```bash
  loops-pp-cli audit lifecycle --agent
  ```
- **`campaigns preflight`** — Check draft status, audience targeting, message presence, and Guardian findings before publication.

  ```bash
  loops-pp-cli campaigns preflight --latest-draft --agent
  ```
