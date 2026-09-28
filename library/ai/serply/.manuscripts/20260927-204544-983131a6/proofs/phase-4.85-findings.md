# Phase 4.85 output review: serply-pp-cli

Reviewed inline (subagents not available in this session) against `scorecard --live-check --json` samples plus extra live calls with num <= 5.

Samples: rank pass, research pass, serp diff skipped by live-check (mutating example needs --allow-destructive; it writes a local snapshot). serp diff exercised by hand in Phase 18.

## Findings

1. format / malformed URLs (warning, FIXED): news results link to news.google.com/rss/articles redirects, so `domain` was news.google.com for every news source. The parser now takes the publisher domain from `source.href` (nature.com, bcg.com). The link stays the redirect because the API returns no direct URL.
2. format (warning, FIXED): Scholar returns `author` as an object (`names`, `authors[]`), so research dropped authors. Parser now maps `author.names`; non-breaking spaces in author and snippet text are normalized.
3. aggregation (warning, FIXED in Phase 12): `/v1/news` ignores `num`; research capped per-vertical results client-side.
4. upstream variance (warning, NOT FIXED): for some queries at num=5, `/v1/scholar` returns cluster links (scholar.google.com/scholar?cluster=...) instead of publisher links; num=3 for the same query returned publisher links. Upstream behavior; the CLI passes the link through.

Relevance (check 1) and ordering (check 4): rank found github.com at position 2 for "open source cli"; research sources for "retrieval augmented generation evaluation" are on-topic (LangChain, arXiv, AWS, ACM, ACL).

---OUTPUT-REVIEW-RESULT---
status: WARN
findings:
- check: format
  severity: warning
  description: news domains were the Google News redirect host (fixed)
  suggestion: use source.href for the publisher domain
- check: format
  severity: warning
  description: scholar author objects dropped (fixed)
  suggestion: map author.names
- check: aggregation
  severity: warning
  description: news ignores num upstream (fixed client-side)
  suggestion: cap per vertical
- check: format
  severity: warning
  description: scholar sometimes returns cluster links at larger num (upstream, not fixed)
  suggestion: disclose; keep num small
---END-OUTPUT-REVIEW-RESULT---
