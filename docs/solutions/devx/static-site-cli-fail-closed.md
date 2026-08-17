---
name: static-site-cli-fail-closed
date: 2026-06-22
problem_type: bug
category: devx
component: library/media-and-entertainment/paul-graham/internal/pg/site.go
root_cause: static-site parsing fallbacks could hide layout drift and full-text search fetched pages serially before honoring limits
resolution_type: fix
tags: [printing-press, static-site, scraper, greptile, cli]
---

# Static-site CLIs should fail closed on parser drift and bound full-text fan-out

## Symptoms

Greptile flagged three issues in the Paul Graham static-site CLI:

```text
SearchFullText makes N sequential HTTP requests for the entire essay index
excerpt slices the string at a byte offset rather than a rune boundary
Brittle HTML heuristics will silently return zero essays or wrong content if the site changes
```

The CLI worked for normal live reads, but the failure modes were real: a layout drift could return empty or wrong data without an actionable error, and `search --full-text --limit 5` could still walk a large essay index serially.

## What didn't work

Relying on `Read` to fall back to `textContent(doc)` was too permissive. It kept the command from erroring, but if the primary content selector broke, callers could receive page chrome or unrelated text as if it were essay content.

Using byte length in `excerpt` looked fine for ASCII-only fixtures, but it could split multi-byte characters once real essay text or future sources include Unicode punctuation or accents.

The title/slug branch in full-text search still called `Read` sequentially for every essay. The `limit` check happened only after appending matches, so broad or late matches could be slow.

## Solution

Fail closed when the index or essay-body parser finds no content:

```go
essays := parseIndex(doc)
if len(essays) == 0 {
	return Index{}, fmt.Errorf("no essays found in %s; page layout may have changed", ArticlesURL)
}

text := extractMainText(doc)
if text == "" {
	return EssayText{}, fmt.Errorf("no essay body found in %s; page layout may have changed", essay.URL)
}
```

Use rune-safe excerpt truncation:

```go
runes := []rune(value)
if max <= 0 || len(runes) <= max {
	return value
}
cut := string(runes[:max])
```

Batch full-text reads with bounded concurrency, keep source ordering, and stop after the first batch that satisfies `limit`.

## Why this works

Static-site CLIs depend on undocumented HTML structure. Returning an explicit layout-change error makes parser drift visible in CI, live smoke tests, and user workflows instead of silently emitting empty data.

Batching search reads caps latency without turning the command into an unbounded scrape. Preserving index order keeps output deterministic.

## Prevention

For future static-site CLIs, add tests for:

- no parsed content returns an error with a layout-change hint
- excerpt output remains valid UTF-8
- live smoke covers at least one representative page read

During review, search for fallback patterns like `if text == "" { text = textContent(doc) }` and for byte slicing in user-visible text truncation.
