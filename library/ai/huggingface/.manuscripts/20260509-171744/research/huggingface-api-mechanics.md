# HuggingFace REST API Mechanics — hf-cli reference

**Scope:** API-mechanics-only. Surface (15 commands, flags, JSON schema) is locked by `docs/plans/2026-05-09-hf-cli-printing-press-seed.md`. This brief fills in HOW each command talks to HF.

**Base URL:** `https://huggingface.co` (override via `HF_ENDPOINT` env to match `huggingface_hub`).
**Auth:** `Authorization: Bearer <HF_TOKEN>` if set, else anonymous. Never required for public-read endpoints listed here.
**Sources checked 2026-05-09:** `https://huggingface.co/.well-known/openapi.md`, `huggingface/huggingface_hub` `hf_api.py` (main), live probes against `bartowski/Qwen2.5-7B-Instruct-GGUF`, `Qwen/Qwen2.5-7B`, `bartowski` user overview.

---

## Endpoints

| Endpoint | Method | Auth | Key params | Key response fields |
|---|---|---|---|---|
| `/api/models` | GET | optional | `search`, `author`, `filter` (repeatable, AND), `sort` (`downloads`\|`likes`\|`lastModified`\|`createdAt`\|`trending_score`), `direction` (`-1` desc, `1` asc), `limit` (default 1000, but server caps; pass small), `full` (bool — include sha/files/tags), `cardData` (bool), `config` / `fetch_config` (bool — include `config.json` parsed), `gated` (bool), `pipeline_tag`, `library` (alias of `filter=library:X`), `language`, `tags`, `trained_dataset`, `apps`, `inference`, `inference_provider`, `num_parameters` (`min:6B,max:128B`), `expand[]` (repeatable, mutually exclusive with `full`/`cardData`/`config` — see expand list below) | JSON array of `{_id, id, modelId, author, sha, lastModified, createdAt, private, gated, disabled, downloads, downloadsAllTime, likes, trendingScore, libraryName, pipeline_tag, tags, siblings:[{rfilename}], cardData?, config?, gguf?, safetensors?}`. Pagination via `Link: <...>; rel="next"` header. |
| `/api/models/{repoId}` | GET | optional | `revision` (path-rev or query — last segment of URL: `/api/models/{id}/revision/{rev}`), `blobs=true` (siblings get `size` + `lfs:{sha256,size,pointerSize}` + `blobId`), `securityStatus=true`, `expand[]=...` (subset/superset control — see list) | All `/api/models` fields plus full `cardData`, `config`, `transformersInfo`, `safetensors:{parameters:{F32,BF16,...}, total}`, `gguf:{total, architecture, context_length, chat_template, bos_token, eos_token, ...}` (only on GGUF repos), `widgetData`, `spaces`, `model-index`, `usedStorage`. |
| `/api/models/{repoId}/tree/{revision}` (or `/tree/{revision}/{path}`) | GET | optional | `recursive` (bool), `expand` (bool — adds last commit data; reduces page size to 100), `limit` (default 1000, 100 with expand), `cursor` (pagination opaque token) | JSON array: `[{type:"file"\|"directory", path, oid, size, lfs?:{oid,size,pointerSize}, xetHash?}]`. LFS sub-object present only on LFS-tracked files. Pagination via `Link: rel="next"` carrying `cursor=...`. |
| `/api/models/{repoId}/treesize/{revision}/{path}` | GET | optional | none | `{size, numFiles}` (folder size in bytes — useful for whole-repo size without enumerating). |
| `/api/models/{repoId}/raw/{revision}/{filepath}` | GET | optional | none | Raw file body. Small text (config.json, README.md, tokenizer.json metadata) — for LFS files this returns the LFS pointer text, not the blob. Use this for `config.json`. |
| `/api/models/{repoId}/resolve/{revision}/{filepath}` | GET | optional | none | Redirects (302) to a CDN URL serving the actual blob (LFS-resolved). Higher rate-limit bucket ("Resolvers" — see Rate limits). Use for downloads, NOT for metadata pokes. |
| `/api/models/{repoId}/blob/{revision}/{filepath}` | GET | optional | none | Same content as `raw` for git-stored files; for LFS, returns blob metadata not raw bytes. Prefer `raw` (text) or `resolve` (binary download). |
| `/api/users/{username}/overview` | GET | optional | none | `{user, fullname, type, avatarUrl, isPro, numModels, numDatasets, numSpaces, numFollowers, numFollowing, numLikes, numUpvotes, numPapers, orgs:[...], createdAt, details}`. 404 if user not found. |
| `/api/whoami-v2` | GET | required | none | Auth probe — returns user object on valid token, 401 anon. Useful inside `doctor` only when `HF_TOKEN` is set. |

**Pagination contract.** HF returns `Link: <next-url>; rel="next"` for `/api/models` and `/tree`. The CLI must follow it explicitly — there is no `next_cursor` field in the JSON body for `/api/models`. For `/tree`, `cursor` is also surfaced as a query param on the `next` link. Accumulate up to `--limit` then stop; never auto-paginate to the end.

**`expand[]` valid values** (model_info / list_models): `author`, `cardData`, `config`, `createdAt`, `disabled`, `downloads`, `downloadsAllTime`, `evalResults`, `gated`, `gguf`, `inference`, `inferenceProviderMapping`, `lastModified`, `library_name`, `mask_token`, `model-index`, `pipeline_tag`, `private`, `safetensors`, `sha`, `siblings`, `spaces`, `tags`, `transformersInfo`, `trendingScore`, `widgetData`, `resourceGroup`. Pass repeated `expand=field` query params. Mutually exclusive with `full`/`cardData`/`config`/`fetch_config` (server returns 400 if mixed).

---

## Per-command mechanics

### Group A — Discovery

**1. `find-quants <base-model>`.** Two passes. (a) `GET /api/models?filter=gguf&search=<basename>&sort=downloads&direction=-1&limit=100&expand=cardData&expand=siblings&expand=tags&expand=downloads&expand=lastModified&expand=author`. Quants are separate repos; HF does not expose `base_model` as a query filter, but cardData carries it (`cardData.base_model` — string or list) and the tag set carries `base_model:<id>` and `base_model:quantized:<id>`. Filter results client-side: keep entries where `cardData.base_model == <base-model>` OR `tags` contains `base_model:<base-model>` OR `tags` contains `base_model:quantized:<base-model>`. (b) For each kept repo, `GET /api/models/{id}?blobs=true` to get siblings with `size` + `lfs.size` (the listing endpoint omits sizes even with `expand=siblings`). Extract quant type from `rfilename` via the regex table below. Apply `--prefer`, `--max-size`, `--uploaders` filters. Uploader rep score comes from in-memory cache of `/api/users/{author}/overview`.

**2. `trending`.** `GET /api/models?sort=trending_score&direction=-1&limit=20&expand=trendingScore&expand=downloads&expand=likes&expand=lastModified&expand=pipeline_tag&expand=tags&expand=library_name`. Apply `--task` → `pipeline_tag=<task>`, `--library` → `filter=library:<lib>` (e.g., `library:gguf`). HF has **no `since` query param** — `--since 7d` is implemented client-side as `lastModified > now-7d`; `trending_score` is already a 24-72h decaying window so for `--since` ≤ 3d the sort alone is usually sufficient. `--size 20b-40b` is post-filter using `safetensors.total` if present, else parsed from tags (`tags` includes `parameters:35B`-style entries on many repos but inconsistently — fall back to model name regex `(\d+(?:\.\d+)?)[Bb]`).

**3. `model-card <id>`.** `GET /api/models/{id}?blobs=true&expand=cardData&expand=config&expand=safetensors&expand=gguf&expand=transformersInfo&expand=siblings&expand=tags&expand=model-index&expand=lastModified&expand=downloads&expand=likes&expand=gated&expand=disabled&expand=private&expand=author`. Active params for MoE: read `config.num_local_experts` (Mixtral/Qwen-MoE) or `config.num_experts` (DeepSeek-style) and `config.num_experts_per_tok`. Active = `total_params * (num_experts_per_tok / num_local_experts)` rounded — but prefer to compute via per-layer expert size if `config.moe_intermediate_size` present (more accurate than ratio). For repos without `config` populated, fall back to `GET /api/models/{id}/raw/main/config.json`. Effective GGUF size from `gguf.total` (already in bytes) when present, else max(`siblings[].size`) for entries matching the GGUF filename regex.

**4. `derivatives <base-id>`.** Same shape as `find-quants` but with broader filter: search models where `tags` contains `base_model:<base-id>` AND `tags` does NOT contain `base_model:quantized:<base-id>` (drop quants). `GET /api/models?search=<basename>&sort=lastModified&direction=-1&limit=200&expand=cardData&expand=tags&expand=downloads&expand=lastModified&expand=author`. Client-side filter on tags (`base_model:<base-id>` AND not `:quantized:`). Sort applied client-side: downloads × recency-decay (e.g., `downloads / (1 + days_since_modified/30)`).

**5. `uploader-rep <user>`.** Two calls. (a) `GET /api/users/{user}/overview` — one shot for `numModels`, `numFollowers`, `isPro`, `createdAt`. (b) `GET /api/models?author={user}&sort=downloads&direction=-1&limit=10&expand=downloads&expand=lastModified&expand=likes` — top-N recent uploads + downloads aggregate. Trusted-uploader flag derived from `numModels >= 20` AND median-downloads-of-top-10 > 1k AND most-recent-upload < 90 days, plus a hardcoded allowlist (`unsloth`, `bartowski`, `mradermacher`, `TheBloke`, `Qwen`, `meta-llama`, `mistralai`, `microsoft`, `google`, `nvidia`).

**6. `compare-quants <id1> <id2> ...`.** N parallel `GET /api/models/{id}?blobs=true&expand=cardData&expand=gguf&expand=safetensors&expand=siblings&expand=tags`. No fan-out shortcut endpoint exists; concurrency cap = 5 (respects 5-min rate-limit window). Joined columns: size (from siblings), quant type (filename regex), uploader (`author`), perplexity (greppable from README via `/raw/main/README.md` — optional pass), MoE active-param ratio (computed as in `model-card`).

**7. `eval-candidates --base <id>`.** Wraps `find-quants <id>` + `model-card` for each, filters by `--target-size` against `gguf.total`/`siblings.size`. No new endpoints; pure composition. Emits harness-input JSON shape from `workspace/scripts/model-eval-harness/` (see harness README — out of scope here).

**8. `find-feature <feature>`.** Two-pass. Pass 1 (broad): `GET /api/models?filter=gguf&sort=downloads&direction=-1&limit=200&expand=config&expand=cardData&expand=tags&expand=siblings`. Pass 2 (per-candidate): when `config` is missing or features not detectable from it, `GET /api/models/{id}/raw/main/config.json` (small text, cheap) and additionally `GET /api/models/{id}/raw/main/README.md` for card-grep features. See "Architecture-feature config.json mapping" table below for detection rules. Honest classification: feature detected in config = "inference-ready"; only in card text = "claimed"; in tags only = "tagged but unverified". Backend-readiness verdict joins detection with bundled `backend-support.json`.

### Group B — Intel loop

**9. `vs-current <id>`.** Local-first. Read `--config-path` (default `./data/openclaw.json`), extract `agents.list[<agent>].model.{primary,fast,long,fallbacks}` (or `agents.defaults.models` if agent uses defaults). Resolve each model id → one `model-card` call (cached per state-dir card-cache TTL 24h). Then one `model-card <id>` for the candidate. Diff: arch (`config.model_type`, `config.architectures`), size (`safetensors.total` or `gguf.total`), license (`cardData.license`), backend support (matrix lookup). Exit 6 with clear message when config path missing — never crash. No new endpoints.

**10. `backend-check <id>`.** Single `GET /api/models/{id}?expand=config&expand=tags&expand=gguf&expand=library_name`. If `config` missing, fall back to `GET /api/models/{id}/raw/main/config.json`. Derive `architecture` from `config.architectures[0]` (e.g., `Qwen2ForCausalLM`, `LlamaForCausalLM`, `MixtralForCausalLM`, `DeepseekV3ForCausalLM`). Cross-reference bundled `backend-support.json` matrix (architecture → {llama.cpp: yes/partial/no, mlx: ..., turboquant: ...} with citation + `source_checked` date). Verdict + cited source out.

**11. `bench-history <id>`.** Pure local. Walk `--harness` dir (default `workspace/scripts/model-eval-harness`) for results JSONL/JSON keyed by HF id. Exit 6 when dir missing. No HF calls.

**12. `watch / watch-poll`.** `watch` is pure state-file mutation (under `flock`). `watch-poll` runs the same query as the watch target's `kind`: `uploader` → `/api/models?author=...&sort=lastModified&direction=-1&limit=20`; `base-model` → `/api/models?search=...&sort=lastModified&direction=-1&limit=50&expand=cardData&expand=tags` (filter client-side by `cardData.base_model`); `feature` → same as `find-feature` but only the cheap pass-1. Diff against `watch-cursor.json`'s last `(id, lastModified)` set; emit new entries. Update cursor only on success.

**13. `local`.** Pure local cache walk. No HF calls (the point is to NOT re-fetch). Cache dirs from `--cache-dirs` flag (default `~/.cache/huggingface,~/.cache/lm-studio/models,/Volumes/models`). Walk `<root>/hub/models--<org>--<name>/snapshots/<rev>/...` and resolve symlinks back to `blobs/<sha>` for actual size on disk. Map directory name back to HF id via `models--{org}--{name}` → `{org}/{name}` (the `--` separator is HF cache-layout convention). Optional one HF call per id to confirm not-deleted, gated behind `--check-remote`.

### Group C — Runtime utility

**14. `doctor`.** One probe: `GET /api/models?limit=1`. Read `RateLimit` and `RateLimit-Policy` response headers (see Rate limits below) → fill `rate_limit_remaining`, `rate_limit_reset_seconds`. `hf_reachable` = HTTP 200/429 reachable, transport error → false. `proxy_in_use` from `HTTPS_PROXY`/`HTTP_PROXY` env. `tty` from `isatty(stdout)`. `state_writable` = touch-test under `flock` then unlink. `has_live_config` = `--config-path` exists + parseable. `has_harness` = `workspace/scripts/model-eval-harness` exists. `backend_matrix_age_days` = `now - max(source_checked)` across bundled matrix. When `HF_TOKEN` set, additionally `GET /api/whoami-v2` to confirm token validity (failure ≠ fail-doctor; just sets `auth_valid=false`).

**15. `schema`.** Pure local. Embeds JSON Schema docs at compile time. No HF calls.

---

## Edge cases

- **Gated models, anonymous request:** `GET /api/models/{gated-id}` returns **401** with body `{"error": "Cannot access gated repo for url ..."}`. With token but user hasn't accepted ToS: **403** with similar body. The model still appears in `/api/models` listings (just with `gated: "auto"` or `gated: "manual"`), so listing is not blocked — only single-card and file fetches are. Map: 401 → exit 6 if token absent ("set HF_TOKEN"), else exit 2; 403 → exit 2 ("model gated, accept terms at huggingface.co/{id}").
- **Private models, no auth:** **401** if URL guessed (HF doesn't leak existence); **404** in listings (filtered server-side). Map both → exit 2.
- **Deleted/disabled models:** `disabled: true` in card response (still 200). Listings exclude disabled by default; `/api/models/{id}` returns the card with `disabled: true` and empty `siblings`. Treat as exit 2 with reason "disabled by HF moderation".
- **Non-existent repo:** **404**, body `{"error":"Repository not found"}`. Exit 2.
- **Revision (branch/tag/commit) not found:** **404** on `/api/models/{id}/tree/{rev}` and `/raw/{rev}/...`. The `/api/models/{id}` endpoint without revision always uses `main`; pass `?revision=` or use the `/api/models/{id}/revision/{rev}` form. Exit 2.
- **GGUF size extraction:** Listings (`/api/models?...&expand=siblings`) return siblings as `[{rfilename}]` ONLY — no sizes. Sizes require `GET /api/models/{id}?blobs=true` (returns siblings as `{rfilename, blobId, size, lfs:{sha256, size, pointerSize}}`). The `gguf.total` top-level field also returns size in bytes when HF parsed the file. Use `gguf.total` first, fall back to `siblings[].lfs.size` for the largest GGUF, then `siblings[].size`.
- **Tree endpoint vs siblings:** `/api/models/{id}/tree/main` always has sizes (no `?blobs=true` needed) and supports `recursive=true`. Prefer it for tree-walks; use `/api/models/{id}?blobs=true` for one-shot card+sizes.
- **HF cache layout (resolve back to id):**
  ```
  ~/.cache/huggingface/hub/
    models--Qwen--Qwen2.5-7B/
      blobs/<sha256-or-git-sha>           # actual content
      snapshots/<commit-sha>/<file>       # symlinks → blobs/...
      refs/main                           # text file: <commit-sha>
  ```
  Directory name regex: `^models--(.+?)--(.+)$` → `{$1}/{$2}`. If org name itself contains `--` (rare), HF escapes — match greedily on the last `--` separator. `lm-studio` cache uses `~/.cache/lm-studio/models/{org}/{name}/{file.gguf}` (flat, no snapshot dir).
- **Trusted-uploader scoring:** Hardcoded allowlist beats dynamic score (Rick's heuristic). Dynamic score for unknowns: `min(1.0, log10(numModels+1)/2) * recency_factor * downloads_factor` where `recency_factor = exp(-days_since_last_upload/180)` and `downloads_factor = min(1.0, log10(median_top10_downloads+1)/4)`. Threshold ≥ 0.5 = trusted, < 0.3 = unknown, ≥ 0.3 < 0.5 = neutral.
- **Rate-limit recovery:** HF returns **429** with `RateLimit: "api";r=0;t=240` (240 seconds until reset). Parse `t=` from `RateLimit` header, clean exit 5 with the duration in the error message. Do NOT auto-retry (matches "no silent retry loop" verification gate). If `Retry-After` header is also present, prefer it (RFC-standard, integer seconds). Token bucket in `rate-limit-bucket.json` decrements on each call; refills based on `RateLimit-Policy: "fixed window";"api";q=500;w=300`.
- **Listing pagination:** `Link: <https://huggingface.co/api/models?...&cursor=XYZ>; rel="next"`. Cursor is opaque; never parse. Stop at `--limit` count regardless of more-pages-available.
- **Search relevance:** `search=foo` matches model id substring (case-insensitive). Multiple words = AND. There is no fuzzy match; misspellings return empty.
- **Empty `config` field:** Many GGUF-only repos lack a parseable `config.json`. `expand=config` will return `null` or absent. Always have a `/raw/main/config.json` fallback path, and tolerate `404` on config (some non-LLM repos don't ship it).
- **`safetensors.total` vs `gguf.total`:** Both in bytes. `safetensors.parameters` is `{F32: int, BF16: int, ...}` (dtype-bucketed param counts). Sum gives total params; multiplied by bytes-per-dtype gives unquantized size. For "active params" on MoE, `safetensors.parameters` is whole-model — use `config.num_experts_per_tok` ratio.

---

## Rate limits

Verified from `huggingface.co/docs/hub/rate-limits` (Sep '25 numbers, "subject to change"):

| Plan | API (per 5min) | Resolvers (per 5min) | Pages (per 5min) |
|---|---|---|---|
| Anonymous (per IP) | 500 | 3,000 | 100 |
| Free user (token) | 1,000 | 5,000 | 200 |
| PRO | 2,500 | 12,000 | 400 |
| Team org | 3,000 | 20,000 | 400 |
| Enterprise | 6,000 | 50,000 | 600 |

**Bucket:** all `/api/*` endpoints we use are in the **API** bucket. Resolvers (much higher cap) is only for `/{id}/resolve/{rev}/{file}` (file downloads, which `hf-cli` does not do). Pages bucket is for HTML browsing.

**429 response headers** (RFC `draft-ietf-httpapi-ratelimit-headers` v9):
- `RateLimit: "api";r=<remaining>;t=<seconds_until_reset>` — read `t=` to know when to retry.
- `RateLimit-Policy: "fixed window";"api";q=<quota>;w=<window_seconds>` — informational, used to size the token bucket.
- `Retry-After: <integer_seconds>` — also returned, RFC-7231 compliant. Prefer this if both present.

**Recovery semantics in `hf-cli`:** persistent token bucket in `rate-limit-bucket.json` shared across all callers (CC, JARVIS, Henry, cron) under `flock`. On 429: write reset-time to bucket, exit 5 with stderr `"rate limited; retry in Ns"`. No silent retry, no exponential backoff loop. `doctor` reads the bucket + last `RateLimit` headers to surface `rate_limit_remaining`. Calls account against bucket before issuing — if remaining=0 and reset in future, exit 5 immediately without making the call.

**Mitigation:** always pass `HF_TOKEN` when present (2× cap). `huggingface_hub` ≥1.2.0 has built-in smart retry that parses `RateLimit` — we deliberately don't replicate this loop (clean exit 5 is the contract).

---

## Cache layout

```
~/.cache/huggingface/                       # HF_HOME default
├── hub/
│   ├── models--<org>--<name>/              # repo dir; -- is separator
│   │   ├── blobs/<sha>                     # real content; sha is git OID for small, lfs SHA256 for LFS
│   │   ├── snapshots/<commit-sha>/<file>   # symlinks into blobs/
│   │   └── refs/<branch-or-tag>            # text file containing the commit-sha
│   ├── datasets--<org>--<name>/            # same shape, ignored by `hf local`
│   └── version.txt                         # cache layout version
├── token                                   # HF_TOKEN if `huggingface-cli login` was run
└── ...
```

**`hf local` walk algorithm.** For each cache root: glob `hub/models--*/`. For each match: parse dir name into `{org}/{name}` (split on last `--`). Read `refs/main` (or any ref file) to get the active commit sha. Walk `snapshots/<sha>/` and resolve symlinks via `os.realpath` → `blobs/<sha>`; sum file sizes (don't count the same blob twice if multiple snapshots reference it). Output: `{id, local_path, size_bytes, files: [...], last_modified}`.

**Other cache layouts to optionally support** (`--cache-dirs`):
- LM Studio: `~/.cache/lm-studio/models/{org}/{name}/<file.gguf>` (flat, no snapshot dir, no symlinks).
- Ollama: `~/.ollama/models/blobs/sha256-<hex>` + `~/.ollama/models/manifests/registry.ollama.ai/library/<name>/<tag>` (registry-style, not HF id-mapped — out of scope unless asked).
- Custom: any dir Rick mounts at `/Volumes/models/`.

---

## Architecture-feature config.json mapping

Used by `find-feature` and `backend-check`. Detection rule = "this combination of fields proves the feature is present in the model architecture (not just claimed in the card)".

| Feature | config.json field/value pattern | Notes |
|---|---|---|
| `moe` | `num_local_experts > 1` OR `num_experts > 1` OR `architectures[0]` matches `/Mixtral\|DeepseekV[23]\|Qwen.*MoE\|JambaForCausalLM\|GraniteMoeForCausalLM/` | Field name varies by family. `num_experts_per_tok` (or `num_experts_per_token`) gives K. |
| `dense` | `num_local_experts` absent AND `num_experts` absent AND `architectures[0]` not in MoE set | Pure negation of `moe`. |
| `mtp` (multi-token prediction) | `num_nextn_predict_layers > 0` (DeepSeek-V3 style) OR `mtp_num_layers > 0` | Inference-readiness is separate (MTP is often training-only). Combine with backend-matrix. |
| `mla` (multi-latent attention) | `kv_lora_rank` present AND > 0 (DeepSeek family) OR `architectures[0]` matches `/DeepseekV[23]/` | MLA is the key differentiator for DeepSeek arch. |
| `gqa` (grouped-query attention) | `num_key_value_heads < num_attention_heads` AND `num_key_value_heads > 1` | Standard since Llama-2 70B. |
| `mha` (multi-head, no GQA) | `num_key_value_heads == num_attention_heads` OR `num_key_value_heads` absent | |
| `mqa` (multi-query) | `num_key_value_heads == 1` | |
| `sliding-window-attn` | `sliding_window` present AND > 0 AND not null | Mistral-style. Some models set it but don't use it — check `use_sliding_window: true` if present. |
| `rope-yarn` | `rope_scaling.type == "yarn"` OR `rope_scaling.rope_type == "yarn"` | Other rope_scaling.type values: `linear`, `dynamic`, `longrope`, `llama3`. |
| `rope-llama3` | `rope_scaling.rope_type == "llama3"` | Llama-3.1+ extended context. |
| `long-context` | `max_position_embeddings >= 32768` | Coarse; cite as "max_position_embeddings ≥ 32k". |
| `tie-word-embeddings` | `tie_word_embeddings: true` | Affects param count vs file size sanity check. |
| `flash-attn-required` | `attn_implementation == "flash_attention_2"` (rare in config; usually runtime) | Mostly irrelevant for inference-readiness. |
| `vision` | `architectures[0]` matches `/.*Vision.*\|.*VL.*\|Llava\|Idefics/` OR `vision_config` key present | |
| `audio` | `architectures[0]` matches `/Whisper\|.*Audio.*\|Qwen2Audio/` OR `audio_config` key present | |

For features ONLY in card text (not config), grep `README.md` content from `/raw/main/README.md` for substrings: `multi-token prediction`, `MTP`, `MLA`, `multi-latent attention`, `YaRN`, `sliding window`, `mixture of experts`. Card-only matches classified as "claimed but unverified".

---

## Quant filename pattern table

GGUF naming convention is loose; bartowski/unsloth/mradermacher are mostly consistent. Apply patterns in order (first match wins).

| Quant type | Regex (case-insensitive, applied to `rfilename`) | Notes |
|---|---|---|
| `F32` | `[-.]F32\.gguf$` | Full precision; rarely shipped. |
| `F16` / `BF16` | `[-.]B?F16\.gguf$` | Half-precision baseline. |
| `Q8_0` | `[-.]Q8_0\.gguf$` | Near-lossless. |
| `Q6_K` | `[-.]Q6_K\.gguf$` | |
| `Q5_K_M` | `[-.]Q5_K_M\.gguf$` | |
| `Q5_K_S` | `[-.]Q5_K_S\.gguf$` | |
| `Q5_0` | `[-.]Q5_0\.gguf$` | Legacy. |
| `Q5_1` | `[-.]Q5_1\.gguf$` | Legacy. |
| `Q4_K_M` | `[-.]Q4_K_M\.gguf$` | Most common daily-use. |
| `Q4_K_S` | `[-.]Q4_K_S\.gguf$` | |
| `Q4_0` | `[-.]Q4_0\.gguf$` | Legacy. |
| `Q4_1` | `[-.]Q4_1\.gguf$` | Legacy. |
| `Q3_K_L` | `[-.]Q3_K_L\.gguf$` | |
| `Q3_K_M` | `[-.]Q3_K_M\.gguf$` | |
| `Q3_K_S` | `[-.]Q3_K_S\.gguf$` | |
| `Q2_K` | `[-.]Q2_K(?:_S)?\.gguf$` | Crunchy. |
| `IQ4_XS` | `[-.]IQ4_XS\.gguf$` | i-quants. |
| `IQ4_NL` | `[-.]IQ4_NL\.gguf$` | Rick's preferred for turboquant. |
| `IQ3_M` / `IQ3_S` / `IQ3_XS` / `IQ3_XXS` | `[-.]IQ3_(M\|S\|XS\|XXS)\.gguf$` | |
| `IQ2_M` / `IQ2_S` / `IQ2_XS` / `IQ2_XXS` | `[-.]IQ2_(M\|S\|XS\|XXS)\.gguf$` | |
| `IQ1_M` / `IQ1_S` | `[-.]IQ1_(M\|S)\.gguf$` | Extreme; use with caution. |
| `UD-IQ4_NL` (unsloth dynamic) | `[-.]UD-IQ4_NL\.gguf$` | Unsloth-specific dynamic quant. |
| `UD-Q4_K_XL` | `[-.]UD-Q4_K_XL\.gguf$` | |
| `UD-Q5_K_XL` / `UD-Q6_K_XL` / `UD-Q8_K_XL` | `[-.]UD-Q[5-8]_K_XL\.gguf$` | |
| `MXFP4` | `[-.]MXFP4(?:_MOE)?\.gguf$` | OpenAI gpt-oss native quant. |
| `split-shard` | `-of-\d{5}\.gguf$` (matches `00001-of-00003.gguf`) | Multi-file repos; treat shards as one logical quant of whatever the prefix indicates. |

**Multi-file (sharded) handling:** when N siblings share a prefix and end `-NNNNN-of-MMMMM.gguf`, sum sizes, treat as one quant entry. Parse the quant tag from the prefix portion (e.g., `Llama-3.1-405B-Q4_K_M-00001-of-00009.gguf` → `Q4_K_M`).

**Imatrix files** (`*.imatrix`): not a quant, sidecar for i-quant calibration. Skip.

---

## Unresolved (flag for /printing-press)

- **`/api/models/{id}/revision/{rev}` form vs `?revision=<rev>` query:** both appear in `huggingface_hub` source paths. Live probe needed to confirm both work for non-main revisions; current brief assumes both do.
- **`tree` endpoint `cursor` token format:** opaque per docs. Not parsed, just round-tripped. No risk surface unless HF changes scheme.
- **`gated: "auto"` vs `"manual"`:** both appear in cards. `auto` = click-through, `manual` = author approval. Not currently differentiated in the CLI spec — may want a hint in `model-card` output later.
