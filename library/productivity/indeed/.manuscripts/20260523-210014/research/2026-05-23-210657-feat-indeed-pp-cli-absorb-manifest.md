# Indeed CLI Absorb Manifest

Scope approved by user (Phase Gate): read-only job search & research. No auto-apply.

## Absorbed (match or beat every existing tool)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Keyword + location search | JobSpy `search_term`/`location` | `indeed-pp-cli search "<q>" --location <l>` | Surf-backed (clears Cloudflare), local store of every hit |
| 2 | Distance/radius filter | JobSpy `distance` | `(behavior in indeed-pp-cli search)` `--radius` | Maps to web `radius` param |
| 3 | Job type filter | JobSpy `job_type` | `(behavior in indeed-pp-cli search)` `--job-type` | fulltime/parttime/contract/internship/temporary → web `jt` |
| 4 | Remote filter | JobSpy `is_remote` | `(behavior in indeed-pp-cli search)` `--remote` | sets `l=Remote`/remote flag |
| 5 | Date-posted filter | JobSpy `hours_old` | `(behavior in indeed-pp-cli search)` `--posted <days>` | web `fromage` (1/3/7/14) |
| 6 | Sort by date/relevance | indeed-scraper `sort` | `(behavior in indeed-pp-cli search)` `--sort date\|relevance` | web `sort` |
| 7 | Pagination / result count | JobSpy `results_wanted`/`offset` | `(behavior in indeed-pp-cli search)` `--limit` / `--page` | pages `/jobs?start=` until limit |
| 8 | Full job description | JobSpy `description{html}` | `indeed-pp-cli job get <key>` | JSON-LD JobPosting + sanitizedJobDescription |
| 9 | Salary parse (min/max/unit/currency) | JobSpy `compensation` | `(behavior in indeed-pp-cli search)` parsed `salary` field | extractedSalary + salarySnippet text |
| 10 | Company info (rating, industry, size) | JobSpy `employer.dossier` | `indeed-pp-cli company <name>` | from job + viewjob companyTabModel |
| 11 | Related/competitor jobs | (Indeed `/m/getcompetitorsjobs`) | `indeed-pp-cli related <key>` | clean JSON endpoint |
| 12 | Location autocomplete | (Indeed autocomplete API) | `indeed-pp-cli locations <q>` | resolve "Austin" → "Austin, TX" |
| 13 | JSON output | JobSpy DataFrame→JSON | `(behavior in indeed-pp-cli search)` `--json` + `--select` | dotted field selection |
| 14 | CSV export | JobSpy DataFrame→CSV | `(behavior in indeed-pp-cli search)` `--csv` | pipeable |
| 15 | Cross-run dedup by job key | JobSpy dedup | `(behavior in indeed-pp-cli store)` INSERT-OR-IGNORE on job.key | durable across runs |
| 16 | Apply URL passthrough | (manual) | `indeed-pp-cli apply <key>` | prints/opens listing URL; never auto-submits |

## Transcendence (only possible with our local-store approach)

| # | Feature | Command | Buildability | Why Only We Can Do This |
|---|---------|---------|--------------|--------------------------|
| 1 | Saved named searches | `saved save <name> ...` / `saved list` | hand-code | Requires persisting query + filters locally |
| 2 | New jobs since last run | `new <name>` | hand-code | Requires `first_seen_at` history per saved search; no API gives this |
| 3 | Offline full-text search | `find "<terms>"` | hand-code | FTS5 over every job ever synced; works with no network |
| 4 | Salary floor across results | `search ... --min-salary N` | hand-code | Requires parsing + filtering Indeed's free-text salary locally |
| 5 | Multi-location fan-out + dedup | `search ... --location "Austin,Dallas,Remote"` | hand-code | Requires fanning N requests and deduping by key in one local pass |
| 6 | Track / shortlist a job | `track <key>` / `tracked` | hand-code | Local shortlist with status; no Indeed equivalent without login |

All 6 transcendence rows are `hand-code` (0 `spec-emits`). The generated typed-endpoint
surface is not used for the headline commands because the data is SSR-embedded JSON parsed
on top of the Surf client.
