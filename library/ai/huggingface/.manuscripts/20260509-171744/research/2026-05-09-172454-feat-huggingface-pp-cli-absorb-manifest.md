# HuggingFace CLI — Absorb Manifest

The seed plan **explicitly inverts away from generic API mirrors** ("Not 'an HF API wrapper' — that's `huggingface_hub`"). So the absorb table is intentionally thin: only the table-stakes lookups every HF tool has. The differentiation lives entirely in the transcendence table — all 15 seed commands are opinionated, stack-aware, novel.

## Existing tool landscape

| Tool | Surface | Why we don't compete with its full surface |
|---|---|---|
| `huggingface_hub` (Python CLI) | Full REST mirror, downloads, uploads, repo management, inference endpoints | We deliberately do NOT mirror; surface is closed by seed |
| `hf-go-cli` (Go, `bdkiran/hugo`) | Search, model details | Pure mirror; no stack awareness |
| `hf` (informal Bash wrappers) | Curl one-liners | Not a competitor; user pain point is no opinionated alternative exists |
| `mlx-lm` / `llama.cpp` model browsers | Backend-specific download helpers | Single-backend; no cross-backend comparison |

No existing tool answers "should I switch from what I run today" or "will this run on my backend" or "has this already been benched." That gap **is** the product.

## Absorbed (table-stakes lookups)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---|---|---|---|
| 1 | Get model card by id | `huggingface_hub` `HfApi.model_info()` | `hf model-card <id>` w/ FTS5-cached store | Active-params loud for MoE; effective GGUF size; offline replay |
| 2 | Search models | `huggingface_hub` `HfApi.list_models()` | Underpins `trending` and `find-quants` | Sort + size-class filter + uploader allowlist baked in |
| 3 | List repo files | `huggingface_hub` `HfApi.list_repo_files()` | Underpins `find-quants` GGUF sibling discovery | `?blobs=true` for sizes; quant-pattern parser |
| 4 | Fetch raw config.json | `huggingface_hub` `hf_hub_download(filename="config.json")` | Underpins `find-feature`, `backend-check`, `model-card` | Cached; arch-feature classifier on top |
| 5 | Authenticated read | `HF_TOKEN` env / cached file | Bearer header + RFC `RateLimit:` parser | Shared rate-limit bucket across CC + JARVIS + cron |

That's it. Everything else is novel.

## Transcendence (only possible with our stack-awareness)

All 15 seed commands. Each is hand-built; none is auto-generated. Score column reflects scorecard fodder weight.

### Group A — Discovery

| # | Feature | Command | Why Only We Can Do This | Score |
|---|---|---|---|---|
| 1 | Quant variant browser w/ uploader rep | `hf find-quants <base> [--prefer iq4_nl,q4_k_m] [--uploaders ...] [--max-size 25g]` | HF doesn't index quants as a category; requires client-side filter on `base_model:quantized:<id>` tag + GGUF sibling parse + uploader rep score | 9 |
| 2 | Trending with size class | `hf trending [--size 20b-40b] [--library gguf] [--since 7d]` | Size-class filter is client-side; HF only sorts by downloads/likes | 7 |
| 3 | Stack-relevant model card | `hf model-card <id>` | MoE active-params (config.json `num_experts_per_tok`), effective GGUF size, training data summary; HF card UI buries these | 8 |
| 4 | Derivative discovery | `hf derivatives <base-id>` | Client-side filter on `base_model:<id>` tag; HF has no first-class derivatives endpoint | 6 |
| 5 | Uploader reputation | `hf uploader-rep <user>` | Aggregates downloads + recency + count + trusted-flag against Rick's allowlist | 7 |
| 6 | Side-by-side quant comparison | `hf compare-quants <id1> <id2> ...` | N×model-card local-join over size, quant, uploader, MoE ratio | 7 |
| 7 | Harness-input emitter | `hf eval-candidates --base <id> --target-size 25g --emit harness-input` | Wires HF discovery into `workspace/scripts/model-eval-harness/` directly. Closes the discovery→eval loop. | 9 |
| 8 | Architecture-feature search | `hf find-feature <feature> [--size ...] [--backend ...] [--moe]` | config.json arch detection (mtp/mla/sliding-window/gqa/rope-yarn/moe) + backend-readiness verdict + wiki pointer. HF doesn't index any of these. | 10 |

### Group B — Intel loop

| # | Feature | Command | Why Only We Can Do This | Score |
|---|---|---|---|---|
| 9 | Replace-current verdict | `hf vs-current <id> [--config-path ...] [--agent main]` | Reads `agents.defaults.models[role]` from `data/openclaw.json`, diffs candidate against running model. NO other tool knows about Rick's stack. | 10 |
| 10 | Backend-readiness oracle | `hf backend-check <id> [--backend llama.cpp,mlx,turboquant]` | Bundled matrix with citations (PR#, commit SHA, wiki note) + `source_checked` dates. Catches "downloaded 30GB then can't load." | 10 |
| 11 | Bench-history join | `hf bench-history <id> [--harness ...]` | Joins HF id with local harness results. Stops re-evaluating known losers. | 9 |
| 12 | Watch + agent-notify | `hf watch <target> [--kind uploader\|base-model\|feature]` + `hf watch-poll` | State-file subscription, cron-callable poll, MC API alert pipeline emit. Passive intel. | 8 |
| 13 | Local cache inventory | `hf local [--cache-dirs ...]` | Walks `~/.cache/huggingface/hub/models--*--*/` + custom dirs, maps to HF ids. Stops accidental re-downloads. | 7 |

### Group C — Runtime utility

| # | Feature | Command | Why Only We Can Do This | Score |
|---|---|---|---|---|
| 14 | Multi-runtime probe | `hf doctor [--json]` | Single-call structured runtime-shape: `{tty, json_supported, state_writable, has_live_config, has_harness, backend_matrix_age_days, rate_limit_remaining, hf_reachable, proxy_in_use}`. Agents branch all calls on this. | 9 |
| 15 | JSON schema introspection | `hf schema [<command>]` | Per-command output schema with `version` field. Agents introspect rather than guess on output drift. | 7 |

**No stubs.** Every command in the manifest is shipping-scope. Stub-marking would defeat the seed's premise.

## Cross-cutting requirements (apply to ALL 15)

Every command MUST implement (per seed runtime portability table):

- `--json` — stable schema + `version: 1`
- `--compact` — strips prose/descriptions, keeps IDs + key fields
- `--explain` — appends one-paragraph rationale
- `--state-dir` flag + `HF_CLI_STATE` env (override default `~/.local/state/hf-cli/`)
- `--no-write` — disables state mutations
- `--limit` — agent-context-sized defaults (10-20)
- TTY-agnostic (no spinners/color when piped)
- Exit codes: `0` success, `2` not-found, `3` backend-incompatible, `4` already-cached, `5` rate-limited, `6` config/state missing
- Output carries `as_of: <ISO8601>`
- Proxy env honored (`HTTPS_PROXY` / `HTTP_PROXY` / `NO_PROXY`)
- Concurrent-write safe (flock on state files)
- Shared rate-limit bucket across all callers

## Bundled data: `backend-support.json`

Embedded in binary. ~30-50 entries. Each entry:

```json
{
  "feature": "moe",
  "backend": "llama.cpp/turboquant",
  "supported": "yes",
  "since": "build-8970",
  "source": "https://github.com/ggerganov/llama.cpp/pull/...",
  "source_checked": "2026-05-02",
  "wiki_pointer": "workspace/jarvis-wiki/cc-ingest/2026-05-02-end-to-end-infra-refresh.md"
}
```

Override via `--backend-support <path>` or `~/.local/state/hf-cli/backend-support.override.json`.

## Wire-ins (post-action gate Q1 — who consumes this?)

- **Rick @ CC** — interactive research, replaces WebFetch dance.
- **JARVIS / Henry / Coulson / codemonkey** — autonomous model-landscape monitoring, candidate sourcing.
- **`workspace/scripts/model-eval-harness/`** — `eval-candidates --emit harness-input` produces direct input format.
- **scan-pipeline** — `watch-poll` becomes a phase in the daily/hourly intel run.
- **MC API alert pipeline** — `watch` notifications use existing 5-field alert schema.
- **JARVIS wiki** — `find-feature` / `backend-check` cite wiki notes (e.g., `apple-silicon-moe-mtp-immature`) as backend-readiness sources.

## Scope reality (for Phase Gate 1.5 conversation)

- 15 hand-built commands × ~80-150 LOC Cobra wiring + RunE = ~1500-2500 LOC
- Bundled `backend-support.json` with citations: ~300-500 lines JSON, agent research effort to populate ~30-50 entries
- Multi-runtime cross-cutting plumbing (TTY detect, exit codes, --json/--compact/--explain wiring, flock, rate-limit bucket, proxy, `as_of` stamps): ~400-600 LOC of `internal/cliutil/` extensions
- State store schema (FTS5 over card cache, watch tables): ~200 LOC
- Bench-history harness reader: ~150 LOC
- Local cache walker: ~150 LOC
- vs-current openclaw.json reader + diff: ~150 LOC

**Total: ~3000-4500 LOC Go** for the printed CLI. Plus README/SKILL/help text. Plus ~100-200 LOC of unit tests for pure-logic packages (quant pattern parser, arch-feature classifier, GGUF size deriver, cache dir parser, watch event differ).

This is firmly on the upper end of what a `/printing-press` run can produce in one session. Realistic wall time: **2-3 hours autonomous build** after Phase Gate 1.5 approval, plus another ~30 min for shipcheck/dogfood/polish.

**Recommended scope conversation at the gate:**

- (a) Full 15 in one run — accept 2-3 hr autonomous build
- (b) Tier 1 first (8 commands: doctor, schema, model-card, find-quants, trending, vs-current, backend-check, local), iterate Tier 2 in a follow-up — ~90 min, validates the pattern, defers `find-feature` / `eval-candidates` / `watch` / `bench-history` / `derivatives` / `uploader-rep` / `compare-quants`
- (c) Tier 1 minus the most novel (e.g. defer `find-feature` and `vs-current` for a separate research-heavy pass) — fastest path to a useful binary
