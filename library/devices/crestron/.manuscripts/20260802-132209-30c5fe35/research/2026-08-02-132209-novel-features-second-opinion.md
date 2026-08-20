# Crestron CLI — Second-Opinion Novel Features Brainstorm

**Status: not acted on. Archived as an independent cross-check.**

A backup brainstorm agent was spawned when the primary one stalled. It returned
after the CLI had already been built, verified, and promoted from the primary
agent's manifest. Its output is preserved here because it is a genuinely
independent pass over the same evidence, which makes it useful input for a
future reprint — not because anything in the shipped CLI changed.

## How it compares to what shipped

Four of its seven survivors are the same features under different names, which
is a reasonable convergence signal for the shipped design:

| Second opinion | Shipped equivalent |
|---|---|
| `fleet check` — paste a model list, get currency + EOL + replacement | `fleet status` (currency) + `lifecycle` (EOL/replacement) |
| `firmware for <model>` — reverse family lookup | the `crestron_release_models` join behind `fleet status` |
| `kit build <file>` — offline bundle for a site | `submittal --out` |
| `successor <model>` — walk the replacement chain | `lifecycle` |

### Where it dissents
It **killed** the 68-row spec comparison that shipped as `specs compare`, calling
it "a formatting exercise on data `product specs` already returns." The primary
agent scored it 7/10 and it shipped. Live testing supports keeping it: comparing
DM-NVX-360 against DM-NVX-363 surfaced 18 differing fields out of 67, including
the Dante-audio distinction that is the actual reason to choose between them.
That is a real answer, not a reformat. Worth revisiting at reprint time if it
sees little use.

### Three ideas not built
Legitimate candidates for a later pass:

1. **`eol watch <file>`** — watch the dated End-of-Sale stream against a saved
   fleet and alert on new hits. The stream is already confirmed dated and
   searchable (category 6, sorted by date), so this is buildable today.
2. **`whatsnew --since <date>`** — diff two snapshots of the local mirror for new
   products, new EOL notices, new firmware, revised docs. Requires retaining
   historical snapshots, which the current schema does not do.
3. **`spec audit <file>`** — re-validate a saved model list against today's
   catalog and EOL stream, flagging drift. A natural extension of `lifecycle`
   over a file input.

Its kill list also independently rejected price tracking (no public data source),
cross-vendor comparison (out of scope), and NL-over-SQL (redundant with the
calling agent) — matching the primary agent's reasoning.

---

## Verbatim output

### Personas
- **Dana Whitcomb, service-contract tech (42 sites, 900+ devices)** — walks in with a spreadsheet of model numbers pulled off a network scan; cannot answer: for these 900 devices, which are on stale firmware, which have hit End-of-Sale, and which have a replacement path.
- **Raul Ferreira, spec writer at a design-build firm** — writes CSI 27 41 00 specs and pastes model numbers into Word from memory; cannot answer: has anything in this 8-month-old spec gone End-of-Sale, been respec'd, or had its spec sheet revised since I wrote it.
- **Kim Oyelaran, Crestron programmer doing pre-truck-roll prep** — packs a laptop for a site with no jobsite internet; cannot answer: which firmware file and which manual do I need on disk for the exact mix of touchpanels at this one site, in one pull.

### Survivors
| # | Feature | Command | Buildability | Score | Persona | Why only we can do this |
|---|---|---|---|---|---|---|
| 1 | Paste a model list (CSV/stdin), get per-device firmware currency + EOL status + replacement rollup | `fleet check` | hand-code | 9/10 | Dana | Needs the family→model many-to-many joined against catalog status, the End-of-Sale stream, and replacement IDs; the site has no page that accepts a list |
| 2 | Watch the dated End-of-Sale stream against a saved fleet and alert on new hits | `eol watch <file>` | hand-code | 9/10 | Dana | EOL is a dated search stream (category 6, "End-of-Sale"); nobody joins it to an installed base but us |
| 3 | Reverse family lookup: model → every firmware release that covers it, newest first | `firmware for <model>` | spec-emits | 8/10 | Kim | Firmware is family-scoped; no single site page answers "which release covers TSS-770" — only a local join does |
| 4 | Diff two snapshots of the local DB: new products, new EOL notices, new firmware, revised docs | `whatsnew --since <date>` | hand-code | 8/10 | Raul | Requires historical snapshots we keep locally; crestron.com only shows current state |
| 5 | Re-validate a saved model list against today's catalog + EOL stream and flag drift | `spec audit <file>` | hand-code | 7/10 | Raul | Joins the user's own spec list to catalog status, EOL dates, and doc revision dates in one pass |
| 6 | One-shot offline bundle for a site: firmware binaries + manuals + spec sheets for a model list | `kit build <file>` | hand-code | 7/10 | Kim | Family fan-out means one release satisfies six models; dedupes downloads no manual process would |
| 7 | Walk the replacement-product ID chain from a discontinued model to its live successor | `successor <model>` | hand-code | 6/10 | Dana | Discontinued items carry a replacement ID; following it repeatedly is a local graph walk, not a page |

### Killed
| Feature | Kill reason | Closest survivor |
|---|---|---|
| Series-level 68-row spec comparison | Nice, but it is a formatting exercise on data `product specs` already returns | `spec audit` |
| Agent-shaped `brief <model>` context dump | Thin wrapper over five commands that already exist; agents can compose them | `fleet check` |
| Firmware update calendar/forecast | Predicting future cadence from a couple years of dates is guesswork, not data | `whatsnew` |
| Full-text search across downloaded PDFs | Needs an OCR/PDF pipeline; huge scope, weak differentiation vs grep | `kit build` |
| Product recommendation ("what should I use for this room") | Needs application knowledge the site does not publish; invents answers | `successor` |
| CAD/Revit bundle export for a room design | Asset download already covers it; bundling adds no join value | `kit build` |
| Price/availability tracking | Not published anywhere public; no data source | — |
| Watch daemon that polls for changes | Packaging, not a feature; same data as a scheduled run | `eol watch` |
| Cross-vendor comparison (Extron/QSC) | Out of scope; no data for other vendors | — |
| Certificate expiry tracker | "Certificates" here are compliance docs, not PKI certs with expiry | `spec audit` |
| Natural-language query over the SQLite DB | The agent calling the CLI already does this; wrapping SQL in NL is redundant | `fleet check` |
| Multi-hop EOL impact ("what else in my fleet depends on this") | Dependency relationships between products are not published | `eol watch` |
