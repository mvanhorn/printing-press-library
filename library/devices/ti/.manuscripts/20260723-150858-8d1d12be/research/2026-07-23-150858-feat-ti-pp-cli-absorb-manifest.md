# TI CLI Absorb Manifest

Deliberately narrow lane CLI (mcmaster-pp-cli precedent: single-purpose, framework
baseline commands retained). Sources absorbed are internal prior art, not public
competitors — no public TI compliance CLI/MCP exists (checked registry + npm/GitHub
during Phase 1 research; TI's official developer API has no compliance fields).

## Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Anonymous RoHS/REACH per OPN | agent repo `TIExtractor` (via login only) | ti-pp-cli part compliance | Works WITHOUT login (product-page JSON-LD); plain HTTP; --json; fail-closed |
| 2 | OPN → generic/report resolution | `tiscrape.ResolveDetailPage` | (behavior in ti-pp-cli part compliance) OPN suffix resolution verified against the page's orderables list | No browser needed for the anonymous lane |
| 3 | Rich Environmental Ratings (RoHS cell + exemptions, REACH cell, SVHC sentence) | `tiscrape.ScrapeRatingsEval` (source) | ti-pp-cli part ratings | Cookie-injected Browserbase session (HIL-deposited cookies), structured JSON, fail-closed on stale cookies |
| 4 | FMD IPC-1752A Class D XML | `tiscrape/fmd.go` (source) + gdx `ti-compliance` cmd | ti-pp-cli part fmd | Saves the Class-D XML (`standardsId=3`) via `/materialcontent/api/opn/pcid/standards`; distinct "login required / cookies stale" error |
| 5 | Blanket CoC/statement PDFs | TI environmental-information page | ti-pp-cli part coc | Downloads applicable signed statements (szzq088/119/087/077/195) as evidence artifacts; manufacturer-CoC pattern per onboarding template |

## Transcendence (only possible with our approach)
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|------------------------|------------------|
| 1 | One-shot lane bundle | part compliance --coc-dir <dir> | hand-code | Emits verdicts + fetches applicable CoC PDFs in one JSON envelope keyed for the extraction lane's manifest `json:` map | none |
| 2 | Cookie-freshness probe | auth check | hand-code | Validates the deposited cookie snapshot against materialcontent WITHOUT running an extraction; distinguishes "stale cookies" from "part not found" | Use before batch runs. Do NOT use to log in — login is HIL via the cowork agent. |
| 3 | Full evidence ladder | part evidence | hand-code | Anonymous verdicts + blanket PDFs always; adds ratings + FMD Class-D XML when cookies work — one command, degrades honestly with per-item status | none |

Scope cut is deliberate (user-approved template): no store/sync/search layer, no
generated endpoint surface beyond these commands, framework baseline (doctor,
which, feedback) retained.
