---
name: full-text-fanout-partial-failures
date: 2026-06-22
problem_type: bug
category: devx
component: library/media-and-entertainment/paul-graham/internal/pg/site.go
root_cause: full-text search treated any per-source FanoutRun error as fatal, discarding successful matches from the same batch
resolution_type: fix
tags: [printing-press, fanout, static-site, greptile, cli]
---

# Full-text fan-out should keep partial matches when one source fails

## Symptoms

Greptile flagged the Paul Graham CLI full-text search as merge-blocking:

```text
SearchFullText treats any per-essay HTTP error as a fatal failure for the entire search.
A single transient network error, 404, or 429 from paulgraham.com while fetching one essay
in a batch causes the function to return nil, err — discarding all partial matches
collected so far.
```

The PR checks were otherwise green, but GitHub still blocked merge because the review
thread remained unresolved.

## What didn't work

The first implementation used `FanoutRun` for bounded concurrency, but then converted
the first per-source error back into a top-level command error:

```go
if len(errs) > 0 {
	return nil, fmt.Errorf("full-text search %s: %w", errs[0].Source, errs[0].Err)
}
```

That defeated the point of `FanoutRun`: it already separates successes from per-source
failures so callers can return partial data while warning about dropped sources.

The first regression fixture also used a tiny fake essay body. That failed the
production `extractMainText` heuristic, which only accepts `td width="435"` cells with
at least 30 spaces, so the test initially produced no matches for the wrong reason.

## Solution

Report per-source fan-out errors to stderr and keep processing successful results:

```go
results, errs := cliutil.FanoutRun(ctx, batch, name, readEssay, cliutil.WithConcurrency(fullTextSearchConcurrency))
if len(errs) > 0 {
	cliutil.FanoutReportErrors(os.Stderr, errs)
}
```

Lock the behavior with a regression test that mixes one 429 response and one valid
essay page in the same search:

```go
matches, err := SearchFullText(context.Background(), essays, "startup", time.Second, 10)
if err != nil {
	t.Fatalf("SearchFullText() err = %v, want nil", err)
}
if len(matches) != 1 {
	t.Fatalf("len(matches) = %d, want 1: %#v", len(matches), matches)
}
```

The same review pass also fixed `read --max-chars` to truncate on rune boundaries:

```go
runes := []rune(text.Text)
if len(runes) > maxChars {
	text.Text = strings.TrimSpace(string(runes[:maxChars])) + "..."
}
```

## Why this works

Static-site full-text search is a best-effort aggregate over many independent page
fetches. A single page returning 404, 429, or a transient network error should not erase
matches already found from other pages. `FanoutReportErrors` keeps the warning visible
while preserving useful results.

Rune-based truncation keeps user-visible essay text valid UTF-8 when output contains
curly quotes, dashes, accents, or other multi-byte characters.

## Prevention

For future static-site or scrape-backed CLIs:

- Treat `FanoutRun` errors as per-source warnings unless the command truly requires
  all sources to succeed.
- Add tests that mix failed and successful sources in the same fan-out batch.
- Make fake HTML fixtures satisfy the same parser heuristics as production pages, or
  the test can fail before it reaches the behavior being verified.
- Search for `text[:limit]` and replace byte slicing with rune slicing for user-visible
  truncation.
