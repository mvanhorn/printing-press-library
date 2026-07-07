# Printing Press Retro: blu-ray (reprint under 4.24.0)

## Session Stats
- API: blu-ray (Blu-ray.com disc catalog)
- Spec source: internal YAML (reused prior research; reprint of 4.8.0 → 4.24.0)
- Scorecard: 92/100 (A) after polish
- Verify pass rate: 100% (verdict FAIL driven only by the R4 data-pipeline false-positive)
- Live dogfood: 88/88
- Fix loops: build re-gen ×1 (R1), shipcheck verify/narrative ×1, dogfood ×3
- Manual code edits: deals-filter restore, binary-envelope unwrap, watch JSON, news-id reject, bluRaySiteURL (polish), 5 store-layer fixes via codex
- Features ported from scratch: 5 novel + catalog store layer

## Findings

### F1 — Novel-feature command colliding with a framework command emits a call to a function the generator deliberately didn't write (Bug)
- **What happened:** research.json `novel_features` had a command named `search`. The framework also emits `internal/cli/search.go` (`newSearchCmd`). The generator warned `novel feature command "search" maps to existing internal/cli/search.go; leaving existing file unchanged` but STILL emitted `rootCmd.AddCommand(newNovelSearchCmd(flags))` in `root.go` — a call to a function it chose not to generate → `undefined: newNovelSearchCmd` → govulncheck/go build gate FAIL. Generation could not complete until the colliding novel was removed.
- **Scorer correct?** N/A (build-time, not a score penalty).
- **Root cause:** `generator` — the novel-command AddCommand wiring is emitted unconditionally even when the generator skips writing the novel function because its name collides with a framework-emitted command.
- **Cross-API check:** `search`, `sync`, `list`, `get`, `watch` are extremely common novel-feature command names AND common framework command names; any printed CLI whose novel feature reuses a framework command name hits this. The build-break is generalizable, not blu-ray-specific.
- **Frequency:** any CLI with a novel command name colliding with a framework command (common — "offline search" is a near-universal novel feature).
- **Fallback if not fixed:** the agent must notice the undefined symbol, diagnose the collision, and drop/rename the novel. A build-break is loud, but it costs a full re-generation cycle each time.
- **Worth a fix?** Yes — emitting a call to a function you deliberately didn't write is an internal inconsistency that breaks the build gate.
- **Durable fix:** when the generator skips writing a novel command file because the name collides with a framework command, it must ALSO skip the corresponding `rootCmd.AddCommand(newNovel<X>Cmd(...))` wiring (the framework command already covers it), or rename the novel deterministically and wire the renamed symbol.
- **Test:** positive — generate a CLI whose research.json novel_features contains `search`; assert `go build ./...` passes and root.go has no dangling `newNovelSearchCmd`. Negative — a non-colliding novel (`bottleneck`) still wires + compiles.
- **Evidence:** first `generate` of this run failed the govulncheck gate with `undefined: newNovelSearchCmd`.
- **Related prior retros:** #2509 (collision-renamed *resources*) — `extends` (adjacent collision handling, but that's spec-resource collision; this is novel-vs-framework-command wiring).
- **Step G case-against:** "the agent should just not name a novel `search`." Fails because the generator already DETECTS the collision (it prints the warning + skips the file) — it just doesn't follow through on the wiring; emitting a call to an intentionally-absent function is a generator bug regardless of naming hygiene.
- **Bucket: Do (P1).**

### F2 — Generator stamps manifest.json version "0.0.0" (Default gap)
- **What happened:** the reprint's `manifest.json` got `"version": "0.0.0"` even though `spec.yaml` declared `version: "0.1.0"`. The prior 4.8.0 generation had stamped the manifest version to the press version (4.8.0). The version ldflag (`-X .../cli.version`) then reports `0.0.0` for the binary.
- **Scorer correct?** N/A.
- **Root cause:** `generator` — manifest version writer defaults to `0.0.0` instead of deriving from the spec `version` (or the press version).
- **Cross-API check:** every CLI generated from an internal YAML spec where the version field isn't separately propagated gets `0.0.0`. Affects all reprints/internal-spec generations.
- **Frequency:** every internal-spec generation.
- **Fallback if not fixed:** agent hand-sets the manifest version + rebuilds with the ldflag (this run did).
- **Durable fix:** stamp `manifest.version` from `spec.version` when present, else the press version; never default to `0.0.0`.
- **Test:** positive — generate from a spec with `version: "0.1.0"`; assert manifest version == "0.1.0" (or press version per policy). Negative — a spec with no version falls back to a sensible non-zero value.
- **Evidence:** promoted manifest had `"version": "0.0.0"`; binary reported `0.0.0` until hand-set.
- **Related prior retros:** #2604 (MCPB manifest name/entry_point from spec title) — `extends` (same manifest writer area, different field). #1978 (canonical-section drift from late manifest fields) — `related-area`.
- **Step G case-against:** "version is user-owned, set it yourself." Fails because the prior generator DID derive it (4.8.0 → 4.8.0); `0.0.0` is a silent regression to a useless default, not a deliberate hand-off.
- **Bucket: Do (P2).**

### F3 — No exported helper to decode the client's binary-response envelope (Missing scaffolding)
- **What happened:** the 4.24.0 client returns non-textual (binary) responses as a base64 envelope `{"_pp_binary":true,"encoding":"base64","data":...}` (`binaryResponseEnvelope`, content-type driven). The struct + `wrapBinaryResponse` are unexported in package `client`. Hand/novel code that fetches binary content (here: gzipped sitemap shards) gets the JSON envelope from `GetWithHeaders` and must re-implement the base64 + `_pp_binary` unwrap itself before it can use the bytes. The ported sitemap pipeline broke at runtime (`gzip: invalid header`) until a local `decodeMaybeBinaryEnvelope` was written.
- **Scorer correct?** N/A.
- **Root cause:** `generator` — the framework wraps binary responses but exposes no public inverse, so every CLI that fetches binary via hand code re-implements the unwrap.
- **Cross-API check:** any CLI fetching gzipped/zip/PDF/image/octet-stream content through hand or novel code. Named with evidence: blu-ray (gzipped `.xml.gz` sitemap shards). Concrete additional shapes are plausible (any sitemap/bulk-export/attachment-download CLI) but only blu-ray is in-hand here → keeps this at P3.
- **Frequency:** subclass — CLIs with hand-coded binary fetches.
- **Fallback if not fixed:** the agent re-writes the unwrap per CLI (small but error-prone; this run's runtime break shows the failure mode).
- **Durable fix:** export `client.DecodeBinaryResponse([]byte) (body []byte, contentType string, ok bool)` (and/or have `GetWithHeaders` optionally return decoded bytes for binary-intent callers). Document it next to `BinaryResponseHeader`.
- **Test:** positive — wrap known bytes via the internal path, decode via the exported helper, assert byte-equality + content-type. Negative — a textual body returns `ok=false` and passes through.
- **Evidence:** `sync` crashed with `gzip: invalid header` until `decodeMaybeBinaryEnvelope` was added in the printed CLI.
- **Related prior retros:** None.
- **Step G case-against:** "only one API in-hand needs it." Real — that's why it's P3, not P2. Survives because the asymmetry (framework wraps but won't unwrap) is a clean, safe, generalizable gap with a tiny fix, and the failure mode is a silent runtime corruption (gzip of an envelope), not a loud build break.
- **Bucket: Do (P3).**

### F4 — verify data_pipeline + dogfood assume the GENERIC generated sync shape (Scorer bug) → COMMENT on #2722
- **What happened:** verify's data_pipeline probe hardcodes the generic generated sync flags (`--db/--resources/--full`) and dogfood's reimplementation check reads `internal/cli/sync.go`. blu-ray ships a hand-authored domain sync (`sync_bluray.go`; flags `--kind/--max-pages/--wait/--quiet`; calls `UpsertCatalogRows`/`UpsertNewsRows`). Result: a false `data_pipeline: "sync crashed"` (verify verdict FAIL) and a dogfood "sync uses generic Upsert only" signal, even though sync works (live dogfood 88/88). Confirmed identical false-positive when running 4.24.0 verify against the PRIOR PUBLISHED copy → harness assumption, not a regression.
- **Scorer correct?** No — the scorer is wrong; the CLI's sync is correct.
- **Root cause:** `scorer` — data-pipeline/reimplementation checks assume the generated sync interface (flag shape + `sync.go` path + generic `Upsert`); a store-backed scrape/HTML CLI with a domain sync diverges legitimately.
- **Cross-API check:** any store-backed HTML/scrape CLI with a hand-authored sync. This is the SAME class already filed as **#2722** ("data-pipeline verify gate false-negatives store-backed CLIs with no syncable JSON resource (HTML/scrape)"). blu-ray adds: the domain-sync flag-shape + `sync.go`-path assumption, and the "confirmed identical on the published copy" evidence.
- **Durable fix:** let the harness detect/accommodate a declared domain-sync shape (e.g. a spec/annotation naming the sync command + its populate path) rather than assuming `--db/--resources/--full` + `sync.go` + generic `Upsert`.
- **Related prior retros / issues:** #2722 (`same` class) → comment; #2707 (reimplementation_check false-positives) `related-area`; #2488 (dogfood live stateful sync pass) `related-area`.
- **Bucket: Do (P3) → comment on #2722, not a new issue.**

## Prioritized Improvements

### P1 — High priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| F1 | Novel/framework command collision emits dangling AddCommand wiring → build break | generator | common (search/sync/list novels) | loud (build fails) but costs a re-gen | small | skip wiring or rename when novel collides with framework cmd |

### P2 — Medium priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| F2 | manifest.json version defaults to 0.0.0 instead of spec/press version | generator | every internal-spec gen | agent often forgets | small | derive from spec.version else press version |

### P3 — Low priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| F3 | No exported decoder for the client binary-response envelope | generator | subclass: hand-coded binary fetch | re-implemented per CLI, silent-corruption failure mode | small | export DecodeBinaryResponse |
| F4 | verify/dogfood assume generic sync shape (domain sync false-FAIL) | scorer | store-backed scrape/HTML CLIs | misleading FAIL verdict | medium | comment on #2722 |

### Skip
| Finding | Title | Why it didn't make it |
|---------|-------|-----------------------|
| F5 | generated client.APIError.Error() embeds the full raw HTTP response body (e.g. nginx 404 HTML wall) | Step G: case-against stronger — the full body aids debugging; truncate-vs-keep is a preference, not a clear bug; only one API (blu-ray) with concrete evidence in-hand. Revisit if a second CLI surfaces noisy HTML-error output. |

### Dropped at triage
| Candidate | One-liner | Drop reason |
|-----------|-----------|-------------|
| bluRaySiteURL hardcoded base host | `sync`/`editions`/`watch` hardcoded `https://www.blu-ray.com/...`, rejected by the SSRF host-guard under a base-URL override | printed-CLI (hand-code bug in the ported novels; fixed in this CLI) |

## What the Printing Press Got Right
- The reprint pipeline preserved + reconciled prior research, novel features, and patch provenance cleanly; regen-merge/promote routing worked.
- verify-skill + validate-narrative caught the dropped deals-filter flags immediately (README/narrative vs source mismatch).
- Live dogfood's mutation-shaped checks caught the watch JSON-fidelity + news-id error-path gaps that build/vet/structural checks missed.
- The binary-response envelope is a genuinely good safety feature (it stops `sanitizeJSONResponse` from corrupting binary bodies) — F3 is only about exposing its inverse, not the design.

## Filed (2026-06-11, mvanhorn/cli-printing-press)
- F1 → #2929 (new, retro/priority:P1/comp:generator)
- F2 → #2930 (new, retro/priority:P2/comp:generator)
- F3 → #2931 (new, retro/priority:P3/comp:generator)
- F4 → comment on #2722 (dedup: same class — data-pipeline false-negative on store-backed HTML/scrape CLIs)
- F5 → Skip (recorded above)
- Artifact upload (catbox) skipped — findings are self-contained inline; manuscripts contain no secrets/PII (public catalog).
