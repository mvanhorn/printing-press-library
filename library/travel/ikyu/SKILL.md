---
name: pp-ikyu
description: Ikyu accommodation discovery in Japan. Use to find hotels or ryokan, inspect exact rooms and plans, compare offers, check alternative dates, or return Ikyu booking links.
argument-hint: "destination or property/room/plan IDs and stay conditions"
allowed-tools: "Read Bash"
---

# Ikyu accommodation

Discover Ikyu hotels and ryokan, inspect exact room-plan conditions, and compare offers with honest price and bath distinctions. Compact results preserve Japanese names, source IDs and freshness.

## Prerequisites: Install the CLI

This skill drives the `ikyu-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install ikyu --cli-only
   ```
2. Verify: `ikyu-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/ikyu/cmd/ikyu-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## Build from a checkout

For an unpublished branch or source checkout, run `go build -o ikyu-pp-cli ./cmd/ikyu-pp-cli` with the Go version in `go.mod`, then add the checkout to this shell’s `PATH`. Verify `ikyu-pp-cli --version` before running recipes.

## When to Use This CLI

Find Ikyu Japan hotels/ryokan, then inspect the exact room and plan before handing booking to the user. The result is a bounded, source-backed shortlist with Japanese names, conditions, prices, uncertainty and canonical links.

## Workflow

1. Establish destination, Japan check-in/check-out dates, adult and A–F child counts **per room**, room count, budget basis and required preferences. Use `stay destinations` for supported source names/paths. Unequal occupancy per room requires Ikyu's booking screen.
2. Run `stay search` with a small limit. Inspect page coverage before widening; advance by `pagination.scanned`, not the filtered result count. Search summaries are not complete room/plan evidence. Unpriced candidates carry explicit gaps and do not prove availability.
3. Run `stay property` for shared facilities and review categories; `stay rooms` for selected-room size, beds, views and bath facts. Each requested preference must be verified or explicitly unknown.
4. Run `stay offer` for the selected property, room and plan. Confirm echoed dates/party, meals, cancellation, conditional price assumptions and availability time. Exact lookup uses the default points variant; preserve a summary’s different variant as an unresolved alternative.
5. Use `stay compare` for up to five exact offers for one stay, or `stay dates` for up to seven explicit dates for one exact room/plan. Present observed price differences separately from compatibility, and preserve item errors.
6. Refresh chosen offers before handoff. Present original Japanese names, known room/plan differences, source amounts with assumptions, unknowns, timestamp and canonical link. Completion means every recommended offer has been inspected or clearly labelled incomplete.

## Unique Capabilities

| Command | Capability |
|---|---|
| `stay compare` | Compare exact offers with explicit compatible groups, differences and unknowns. |
| `stay dates` | Inspect bounded date alternatives for one selected room and plan. |
| `stay rooms` | Find rooms with source-backed space and bath attributes; keep missing evidence visible. |
| `stay offer` | Separate source prices, points scenarios, coupons and eligibility for an exact offer. |
| `stay search` | Return compact dated property candidates with pagination, freshness and detail gaps. |

## Command Reference

| Intent | Command |
|---|---|
| Resolve a destination | `stay destinations` |
| Build a dated property shortlist | `stay search` |
| Inspect property/review categories | `stay property` |
| Inspect rooms and plan summaries | `stay rooms` |
| Inspect exact conditions | `stay offer` |
| Compare selected offers | `stay compare` |
| Inspect date alternatives | `stay dates` |

Use command `--help` for flags. Use `--dry-run` to inspect a request plan without source access. Prefer this `stay` workflow over the generated framework's generic local-store `search`.

## Recipes

### Dated shortlist

```bash
ikyu-pp-cli stay search --destination tokyo --check-in 2026-11-17 --check-out 2026-11-18 --adults 2 --limit 3 --agent --select data
```

Keep bounded source summaries and explicit coverage.

### Room evidence

```bash
ikyu-pp-cli stay rooms 00002889 --check-in 2026-11-17 --check-out 2026-11-18 --adults 2 --limit 3 --agent
```

Inspect room-level bath and bedding evidence.

### Exact offer

```bash
ikyu-pp-cli stay offer 00002889 10193727 11055986 --check-in 2026-11-17 --check-out 2026-11-18 --adults 2 --agent
```

Check cancellation, meals and conditional price amounts.

## Auth Setup

Verified public accommodation reads are anonymous and require no API key. Personalized member inventory and coupon eligibility are outside this CLI; final booking stays on Ikyu.

Run `ikyu-pp-cli doctor` to verify setup.

## Agent Mode

`stay` commands default to compact JSON. `--agent` keeps execution non-interactive; `--select` narrows output. Freshness, pagination and partial errors are part of the evidence, so preserve them when answering. Diagnostics belong to stderr. Cache hits are prior observations; use `--refresh` for current availability. Stale data requires explicit opt-in.

## Interpretation

- Keep property, room, plan and dated offer separate. Preserve source Japanese names and stable string IDs.
- Treat outdoor/private/hot-spring room baths as independent facts. Shared facilities never establish room amenities.
- Keep headline amounts, conditional payable, points earned/applied, coupon discounts and eligibility separate. Earned and immediately applied points are alternative scenarios; subtract neither twice. Source JPY totals cover requested rooms/nights; extra taxes can remain. Unknown checkout payable stays unknown. Anonymous eligibility remains unverified, so live comparisons do not assert equivalent-price savings.
- Compare only known compatible dates, occupancy, room, meals, cancellation and payment/price eligibility. Two missing facts do not prove equivalence. Different properties/rooms/dates are alternatives, not equivalent-offer savings.
- For field interpretation, child/cancellation categories or access caveats, read [source contract](docs/source-contract.md).

## Boundaries

Use Ikyu's website for reservation/payment/cancellation actions, account-specific offers and uneven room occupancy. Use other tools for restaurant/spa bookings, overseas accommodation and other OTAs. Source denial or schema failure is an error, not no availability.

## Output Delivery

Return a concise shortlist with canonical links and the conditions needed to assess it. Report partial failures and missing attributes next to affected offers. Explain the selected room/plan from source facts; avoid an invented universal hotel score.
