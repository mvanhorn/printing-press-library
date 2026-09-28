---
name: pp-tabelog
description: "Tabelog restaurant and bar discovery in Japan, restaurant-page inspection, and offline trip shortlists with comparison and freshness checks."
author: zjsng
license: Apache-2.0
allowed-tools: "Read Bash"
---

# Tabelog

Use `tabelog-pp-cli` from PATH, or `./tabelog-pp-cli` in a built checkout. Build instructions: [README.md](README.md). Public English sources need no account or API key.

## Prerequisites: Install the CLI

This skill drives the `tabelog-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

This CLI uses a local Go module. Run the following from the root of this source checkout (requires Go 1.26.6 or newer):

```bash
go install ./cmd/tabelog-pp-cli
```

Go installs the binary into `$GOBIN` when set, otherwise `$GOPATH/bin` (default `$HOME/go/bin`). Add that directory to `$PATH` for the agent/runtime that will invoke this skill, or invoke the installed binary by its full path.

Verify: `tabelog-pp-cli --version`

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## Workflow

1. Resolve a location with `areas` and a category with `cuisines`. Use returned typed choices when names are ambiguous.
2. Use `find` for a small source-ranked candidate set; inspect selected canonical URLs or cached IDs with `show`.
3. Save fetched candidates with `lists add`. Maintain notes and membership with `lists show`, `lists note`, and `lists remove`.
4. Compare saved facts offline. Audit missing or old evidence before refreshing selected IDs.

Completion means structured results and their provenance support the answer. Use command help for current flags instead of loading the full command catalog.

## Compact discovery

```bash
tabelog-pp-cli find --area tokyo --cuisine bar --meal dinner --budget-max 5000 --limit 5 --agent --select items.id,items.name,items.rating,items.review_count,items.dinner_budget,items.url
```

Read `items` for domain results and `meta` for effective criteria, source, age and coverage. `--agent` retains meaningful facts in compact JSON. Projection narrows fields while retaining essential metadata. A source next-page link is not a cursor immediately after the last displayed item.

Find summaries share identical `fetched_at` and `source_surface` values in `meta`; a row value overrides that default. `meta.budget_source` supplies an omitted meal-budget source; an explicit budget `source` overrides it. Explicit projection reads the full records.

## Unique Capabilities

| Command | Use it for |
|---|---|
| `lists add` | Persist fetched candidates and personal notes |
| `lists compare` | Compare saved facts offline |
| `lists refresh` | Fetch current details and report changes |
| `lists alternatives` | Match saved backups against explicit constraints |
| `lists audit` | Find unfetched, unknown or old evidence |

For saved-candidate planning, read [docs/saved-lists.md](docs/saved-lists.md).

## Accuracy and boundaries

- Preserve ratings, review counts, separate meal budgets, links and retrieval times. Tabelog scores are weighted source signals, not universal quality verdicts.
- Budget brackets are source estimates. Station meters are relative to the named station. Reservation policies do not establish current availability.
- Distinguish source-unknown facts from details not fetched. Report bounded coverage, stale data and partial refresh failures. `--data-source local` makes zero source requests; a cache miss does not mean no restaurants exist.
- On failure, inspect stdout for usable partial results and stderr for diagnostics. Domain errors are JSON under `--agent`; global flag-parser errors can be text.
- Booking, payment/account changes, live availability/open-now conclusions, walking routes and bulk review/image harvesting are outside this CLI's scope.
