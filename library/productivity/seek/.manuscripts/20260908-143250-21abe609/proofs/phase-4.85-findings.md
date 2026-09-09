# SEEK CLI — Phase 4.85 Agentic Output Review Findings

Run: 20260908-143250-21abe609 · reviewer status: **WARN** (Wave B — warnings, non-blocking)

Samples reviewed: `salary`, `trends`, `company`, `listings facets`, `classifications`.
`me new-jobs` not assessable (correctly failed on missing auth).

Checks 1 (semantic relevance), 2 (format bugs), 3 (source aggregation, N/A) — PASS.

## Finding 1 — `salary` distribution distorted by outliers (warning) — FIXED

**Reviewer:** salary "registered nurse" histogram spanned ~$18k floor to ~$790k
top, 6 of 8 buckets near-empty, p90 ~$233k implausible for the role.

**Root cause:** the parser annualises correctly, but the sample legitimately
mixes casual / part-time roles (genuine ~$18–40k annual) with senior/exec
nursing packages, plus the occasional data-entry outlier. Raw percentiles and
an equal-width histogram over that spread look broken.

**Fix applied:** `seekparse.Winsorize` clips the parsed-salary set to
[p2.5, p97.5] before the histogram and the p10–p90 percentiles are computed.
Extreme tails no longer stretch the bins or skew p90. The raw sample size and
disclosure rate are still reported unmodified. `salary_test.go` covers it.

## Finding 2 — `company --active` is a near no-op (incidental note)

**Reviewer:** `company --active` produced byte-identical output to `company`.

**Assessment:** expected. SEEK's search endpoint only returns currently-listed
ads, so the `--active` filter (drop listings older than 30 days) rarely trims
anything — SEEK auto-expires at 30 days. The flag was in the approved Phase 1.5
manifest and is genuine defensive filtering for the rare near-expiry ad.
**Action:** tightened the cutoff to 28 days and documented it as "rarely changes
the result set" in the flag help. Not removed (manifest-approved).
