# Retraction Checker — Shipcheck Proof

## Shipcheck umbrella: PASS (7/7 legs)
| Leg | Result |
|-----|--------|
| verify | PASS |
| validate-narrative | PASS |
| dogfood | PASS (4/4 novel features built) |
| workflow-verify | PASS |
| apify-audit | PASS |
| verify-skill | PASS |
| scorecard | PASS — 87/100, Grade A |

## Live behavioral verification (keyless, real APIs)
| Command | Test | Result |
|---------|------|--------|
| check | DOI 10.1016/j.micpro.2020.103768 (retracted) | RETRACTED, type=retraction, date=2021-03-01, source=publisher, notice URL — correct |
| check | DOI 10.1038/nature12373 (clean) | not retracted — correct |
| check | PMID:9500320 (Wakefield, retracted) | resolved PMID→DOI via NCBI esummary, RETRACTED via title-prefix — correct |
| scan | .txt with #comment + blank lines + 3 DOIs | 3 parsed, 1 retracted flagged, 0 errors — correct |
| watch | "crispr" first run then second run | baseline established (10 notices), second run 0 new — correct diff behavior |
| superseded | 10.1016/j.micpro.2020.103768 | Crossref lookup OK; OpenAlex leg blocked (see baseline) |

## Exit codes
- check no-arg → 2 (usage); scan missing-file → 1; happy paths → 0. Correct.

## Known external baseline (NOT a CLI bug)
OpenAlex is currently returning HTTP 503 for all anonymous search
("Anonymous search is temporarily rate-limited due to heavy load"). A raw curl
to the identical, well-formed URL the CLI builds also returns 503, confirming
the outage is upstream and global, not a request-construction bug. The
`superseded` command:
- builds a verified-correct OpenAlex request (search + sort=cited_by_count:desc + from_publication_date filter),
- retries 503/429 up to 3x with linear backoff,
- surfaces a clear, actionable error rather than phantom-empty results.
Re-run `superseded` when OpenAlex recovers to confirm the happy path.

## Notable fix during dogfood
- PMID→DOI resolution initially used the NCBI PMC ID Converter, which only
  covers the PMC open-access subset (Wakefield PMID returned no DOI). Switched
  to NCBI E-utilities `esummary` (covers all of PubMed). Verified end-to-end.

## Verdict: ship (pending OpenAlex recovery re-test of superseded happy path)
Not yet promoted to library or published — awaiting user confirmation.
