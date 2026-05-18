# PCGS Novel Features Brainstorm — Full Subagent Output

## Customer model

**Vinny — the upstream-collector / single dogfood user.**

- **Today (without this CLI):** Vinny owns a growing pile of PCGS-graded slabs and a curated `pcgs-coin-list.csv` plus a `test-input-generated/` folder of mixed-shape fixtures (CSV, JSON wrappers, JSONL, plain text, plus-grade slab IDs). To verify a cert he either pastes it into pcgs.com/cert one at a time or curls the public API with his bearer token and hand-parses the envelope. His personal collection system lives downstream — he needs *raw* PCGS records pushed to it, not curated ones.
- **Weekly ritual:** Once a week he ingests new slab IDs (mostly from auction wins / mail-day photos) by feeding a CSV/JSONL into the same process and re-verifies a slice of his existing collection to refresh Population/PriceGuide/Auction data. The constraint that frames every session is the 1000-call/day hard cap — he plans batches around it.
- **Frustration:** No community wrapper exists at all. Every batch he runs today is a hand-rolled loop with no quota awareness, no idempotency, no separation between immutable identity fields and mutable market data, and no durable cache — so the same cert gets re-fetched, burning quota, and a transient bad response can corrupt cert identity fields in his downstream store.

**The numismatic dealer — population/price-curve watcher.**

- **Today:** Looks up auction realized prices and PCGS population by clicking through CoinFacts pages and copying numbers into a spreadsheet. To build a population curve for one PCGSNo across grades 1-70 + Plus, he'd have to make 70+ ad-hoc clicks or curl calls.
- **Weekly ritual:** Picks the half-dozen series he's making a market in and refreshes pop + PriceGuide + recent APR for the grade bands he holds inventory at. Compares scarcity (PopHigher vs Population) to decide what to bid on at the next auction.
- **Frustration:** No way to ask "give me the whole pop curve for PCGSNo X in one shot," no way to spot when Population drifts (more got graded), and the public site has no "show me only what changed since last week" mode.

**The submitter — tracking grading jobs in flight.**

- **Today:** Has one or more open PCGS submissions and refreshes the My-PCGS site to see status. Once a submission posts, copies each cert into his records.
- **Weekly ritual:** Polls submission numbers a few times a week; on completion, fans out cert-by-cert to grab CoinFacts and images for the freshly graded coins.
- **Frustration:** Has to manually thread submission → cert list → per-cert lookup → image pull. Nothing chains them.

## Candidates (pre-cut)

| # | Candidate | Command | One-line | Persona | Source |
|---|-----------|---------|----------|---------|--------|
| C1 | Quota status & forecast | `quota` | Show today's used/remaining/reset, plus "this planned batch would need N calls — fits / would blow budget" | Vinny | (a),(e) |
| C2 | Dry-run cost estimator | `coin batch --file X --dry-run` | Parse the fixture, dedupe vs cache, print "would make N live calls / M cache hits" without spending quota | Vinny | (a),(e) |
| C3 | Mutable-only sync | `sync --max-age 7d [--cert … \| --all]` | Refresh ONLY Population/PopHigher/PriceGuideValue/AuctionList/Images/CoinFactsNotes on cached rows; never touch identity fields; emit a diff | Vinny | (a),(b),(e) |
| C4 | Immutable-drift sentinel | `verify-identity --cert <c>` | Re-fetch and assert every identity field equals the cached row; non-zero exit + per-field diff if PCGS contradicts itself | Vinny | (a),(b),(e) |
| C5 | Spec-number population sweep | `coin pop-curve <pcgs-no> [--plus]` | One command pulls every grade 1-70 (+Plus) for a PCGSNo, writes the full pop curve to local store, prints scarcity table | Dealer | (a),(b),(c) |
| C6 | Pop-shift watcher | `coin pop-shift <pcgs-no> --since <date>` | Diff cached pop-curve snapshots | Dealer | (b),(c) |
| C7 | Cert-list ingest from VISION fixtures | `coin ingest <path>` | Auto-detect CSV/JSON wrappers/JSONL/plain text → cert list | Vinny | (a),(e) |
| C8 | Lookup-log audit | `audit [--since 7d] [--endpoint …]` | Pure SQL over `lookup_log` | Vinny, Dealer | (c),(e) |
| C9 | Submission fan-out | `order hydrate <submission-no>` | Submission → per-cert CoinFacts + Images, quota-aware | Submitter | (a),(b) |
| C10 | APR window report | `coin apr-window <pcgs-no> --grade <g> --since 90d` | Local stats over APR | Dealer | (b),(c) |
| C11 | Melt-vs-price advisor | `coin melt <cert>` | MetalContent + Weight + user-supplied spot prices | Dealer | (b) |
| C12 | Local search across cached coins | `search --text "morgan" --year 1881 --min-pop 100` | FTS + numeric filters | Vinny, Dealer | (c) |
| C13 | Resumable cache-warm | `coin batch --file pcgs-coin-list.csv --resumable` | Checkpointed batch that can span UTC days | Vinny | (e) |
| C14 | Reset-window probe | `quota probe` | Confirm reset window | Vinny | (e) |
| C15 | Images bulk-pull | `order images <submission-no>` | Folds into C9 | Submitter | (b) |

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Buildability | How It Works | Evidence |
|---|---------|---------|-------|--------------|--------------|----------|
| 1 | Quota status & forecast | `quota [--for <file>]` | 10/10 | hand-code | Reads `lookup_log` for today's UTC window, computes used/remaining/reset; when `--for` supplied, parses the fixture, dedupes against cache, and projects required call count | VISION.md mandates 1000/day hard cap with quota command + 80%/95% warnings; brief Top Workflow 4 |
| 2 | Dry-run cost estimator | `coin batch --file <f> --dry-run` | 10/10 | hand-code | Same fixture parser as `coin ingest` + cache-hit lookup against local `coin` table; emits "N live calls, M cache hits, would consume X% of remaining quota" without spending a call | VISION.md 1000/day constraint + brief Top Workflow 2 |
| 3 | Mutable-only sync | `sync [--cert … \| --all] [--max-age 7d]` | 10/10 | hand-code | For each in-scope cached row, re-calls `GetCoinFactsByCertNo` (and APR/Images if requested), updates ONLY Population/PopHigher/PriceGuideValue/AuctionList/Images/CoinFactsNotes columns, leaves cert identity fields untouched, emits per-field diff | VISION.md "Mutable-only sync" + brief Top Workflow 3 |
| 4 | Spec-number population sweep | `coin pop-curve <pcgs-no> [--plus] [--grades 1-70]` | 9/10 | hand-code | Iterates grades 1-70 (+Plus when `--plus`) calling `GetCoinFactsByGrade`; writes `(PCGSNo, GradeNo, PlusGrade) → Population/PopHigher` rows to local store and prints the curve as a table | brief Top Workflow 5; PCGS-specific 1-70 + Plus grade axis |
| 5 | Cert-list ingest from VISION fixtures | `coin ingest <path> [--format auto]` | 10/10 | hand-code | Auto-detects CSV/JSON `{slabs:[…]}`/JSON `{holdings:[…]}`/JSONL/plain text; normalizes plus-grade slab IDs; emits a clean cert list to stdout that `coin batch` and `sync` can consume | brief: "Local fixtures (`test-input-generated/`) are first-class" — 11 valid + 7 deliberately-bad fixture shapes already exist |
| 6 | Lookup-log audit | `audit [--since 7d] [--failed] [--endpoint …] [--by-day]` | 8/10 | hand-code | Pure SQL over the `lookup_log` table; supports `--failed`, `--by-endpoint`, `--by-day`, `--cert <c>` | brief Data Layer explicitly defines `lookup_log` |
| 7 | Submission fan-out | `order hydrate <submission-no> [--with-images]` | 7/10 | hand-code | Calls `GetOrdersBySubmissionNo`, extracts every CertNo, fans out per-cert `GetCoinFactsByCertNo` (and `GetImagesByCertNo` when `--with-images`), respecting cache and refusing to start if remaining quota < cert count | submitter persona's manual fan-out |
| 8 | Local search across cached coins | `search --text <q> [--year …] [--min-pop …] [--max-price …]` | 9/10 | hand-code | FTS5 over Name/Country/SeriesName/Category/Designer/MintLocation/MajorVariety/MinorVariety/DieVariety + range filters on Year/Grade/PriceGuideValue/Population/PopHigher/Mintage/Weight/Diameter | brief Data Layer explicitly lists these FTS + range columns |
| 9 | Resumable cache-warm | `coin batch --file <f> --resumable [--checkpoint <path>]` | 9/10 | hand-code | Extends absorbed `coin batch` with a checkpoint file recording last-completed cert; on resume, skips completed certs and stops cleanly when quota reset is needed (split a 1000-row CSV across multiple UTC days) | VISION 1000/day hard cap + `pcgs-coin-list.csv` |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| C4 verify-identity | Thin wrapper around `coin facts cert` + assert; identity-drift detection is better expressed as a flag on `sync` that fails when an immutable field would change | C3 `sync` |
| C6 pop-shift | Useless on first print (no prior pop-curve snapshots to diff); reschedule as a P3 follow-up | C5 `coin pop-curve` |
| C10 apr-window | Borderline wrapper over `GetAPRByGrade` with local count/min/max/median — too thin to stand alone; ship the stats as a `--stats` flag on the absorbed `coin apr grade` during P3 | absorbed `coin apr grade` |
| C11 melt | Niche (only bullion-content coins); dealer persona already has melt math; spot prices have to be passed by flag, which makes it a calculator | (none) |
| C14 reset-window probe | One-shot diagnostic, not a weekly action; the same evidence is captured naturally by `audit` once real calls run near a UTC boundary | C6 `audit` |
| C15 images bulk-pull | Same orchestration as `order hydrate` with one extra endpoint per cert; folds in as `--with-images` | C7 `order hydrate` |
