# Phase 4.85 — Agentic Output Review Findings (`meta-ads-library-pp-cli`)

- **Run ID:** `20261127-082434-b7f7d73b`
- **Review Date:** 2026-11-27
- **Surface Reviewed:** Live zero-auth GraphQL public Meta Ad Library commands (`advertisers search`, `ads search`) and local SQLite competitive intelligence commands (`winners`, `hooks`, `funnels`, `sov`).

## Live Sample Probes Executed

| Command | Mode | Exit | Result Quality |
|---|---|---:|---|
| `advertisers search "Nike" --country US --limit 3 --agent` | live | 0 | Returned official Nike Page (`15087023444`, likes, IG handle, verification) in compact JSON array. |
| `ads search --page-id 15087023444 --country US --limit 10 --agent` | live | 0 | Returned 10 active commercial ads, unwrapped redirect links into `nike.com` landing URLs, persisted into local SQLite + recorded snapshot. |
| `winners 15087023444 --min-days 30 --limit 5 --agent` | local | 0 | Ranked Nike's 525-day evergreen creative #1 with exact `days_running`, `collation_count`, `winner_score`, `opening_hook`, and `landing_domain`. |
| `hooks 15087023444 --limit 5 --agent` | local | 0 | Extracted and deduplicated opening copy hooks weighted by collation scale and max days running. |
| `funnels 15087023444 --limit 5 --agent` | local | 0 | Grouped unwrapped `nike.com` product landing pages and UTM funnels across active creatives and carousel cards. |
| `sov "nike" --limit 5 --agent` | local | 0 | Ranked Nike #1 with `ad_share_pct`, `collation_share_pct`, `dominant_format`, and `median_days_running`. |

## Findings & Disposition

1. **DPA Template Placeholders in Upstream Creative Titles (Informational — No Bug):** Dynamic Product Ads (DPAs) return literal `{{product.name}}` in Meta's upstream `snapshot.title` field. Preserved verbatim as authentic upstream metadata.
2. **Recipe 5 `--agent` Projection Alignment (Fixed):** Official `archive` recipe previously omitted `--select`, which caused `--compact` under `--agent` to drop `eu_total_reach`, `spend`, and `impressions`. Updated Recipe 5 across `research.json`, `README.md`, and `SKILL.md` to include `--select id,page_name,eu_total_reach,spend,impressions,currency`.
3. **Output Description Fidelity (`sov`) (Fixed):** Updated `sov` descriptions across `research.json`, `sov.go`, `README.md`, `SKILL.md`, `root.go`, `which.go`, and `.printing-press.json` from `"format mix"` to `"dominant format"` to match the emitted `dominant_format` field.

## Verdict

**PASS** — All sampled live and local commands produce domain-accurate, compact, schema-consistent JSON output with zero broken fields or empty placeholders.
