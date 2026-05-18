# podcast-goat — Novel Features Brainstorm (audit trail)

Spawned 2026-05-17 against the absorb manifest of 18 absorbed features.
First print (no prior research). Subagent ran the standard three-pass pipeline.

## Customer model

### Lan Xuezhao (@xuezhao) — investor running Hermes research agents

**Today.** She is the canonical user the brief is built around. She pays for Huberman,
Acquired, Founders, and Peter Attia memberships and reads (or has Hermes read) 6-10
long-form interviews a week looking for thesis fodder. The reference output for everything
she wants is the chip-supply-chain.fly.dev page — two full Dwarkesh transcripts pasted in
by hand and processed by an agent. She copy-pastes from logged-in member pages into a
Claude window because no tool will use her cookies.

**Weekly ritual.** Monday she opens five member tabs (Huberman recap, latest Acquired drop,
two Dwarkesh posts, one Founders), copies the speaker-labeled HTML into Hermes one at a
time, and pastes the same "now synthesize across these" prompt. The bottleneck is not the
agent, it is the manual fetch. She has never used yt-dlp. She wants one command per URL
and a "give me everything I have on chip supply chains" bundle command.

**Frustration.** She subscribes legitimately and still cannot programmatically pull her
own transcripts. Public scrapers ignore member auth, paid APIs (spoken.md, Taddy) charge
her for episodes she already owns, and the speaker labels are inconsistent across sources
so her Hermes prompts have to special-case each one.

### Hermes / Claude Code agent (the silent operator)

**Today.** The agent runs inside a chat session, gets a topic ("the chip supply chain"
or "what does Senra say about Buffett's compounding"), and has to assemble a corpus.
Without podcast-goat it either makes up source material from memory or asks Lan to paste
transcripts. With it, the agent calls `episode get` and `magic` over MCP, gets canonical
markdown back, and does the synthesis it is actually good at.

**Weekly ritual.** Dozens of `episode_get` and `episode_search` MCP calls per session.
The agent does not care about humans; it cares about deterministic schema, predictable
cost, and clear `mcp:read-only` annotations so Lan does not get spammed with permission
modals.

**Frustration.** Today there is no MCP server that returns speaker-labeled transcripts
in one shape. The agent gets HTML from one source, VTT from another, raw JSON from a
third, and has to normalize each itself, wasting context on parsing instead of thinking.

### Trevin Chow — founder doing competitive teardowns

**Today.** Trevin listens to Acquired and Founders on long runs and remembers
half-citations like "Senra talked about Rockefeller's pricing power somewhere." He wants
to grep that line back out in 5 seconds, not 5 minutes. He has no Huberman subscription
but does subscribe to Acquired. He uses Claude Code daily and would happily invoke a
slash command from his editor.

**Weekly ritual.** Two 60-min episodes a week. Occasionally pulls a specific quote into
a doc or Slack thread. Does not run an agent harness; uses the CLI directly with `episode
search` and `episode get`. Cares less about magic bundles and more about exact-text
recall over the local cache.

**Frustration.** Existing transcript tools cache nothing across sources; he cannot search
Acquired AND Founders AND Huberman with one query. Granola-style local search exists for
his own meetings but not for the podcasts he listens to.

### Zhang Xiaojun listener (multilingual researcher)

**Today.** Chinese-speaking, English-reading. The Xiaojun show is YouTube-only with
zh-Hans auto-subs and no English transcript. Today they paste the YouTube URL into a
translator extension and copy line by line.

**Weekly ritual.** One Xiaojun episode every 1-2 weeks. Wants both zh-Hans original AND
English auto-translation in one markdown file so they can quote either side into a
bilingual note. Will not pay $0.08/ep when yt-dlp gives them auto-subs for free.

**Frustration.** Every transcript CLI assumes English. yt-dlp works but the subtitle
parsing + alignment is a 30-line shell incantation they do not want to maintain.

## Candidates (pre-cut)

[full 16-candidate list preserved — see survivors + killed-candidates tables below for selection]

## Survivors and kills

### Survivors

| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|--------------|----------|
| 1 | Topic bundle for one-shot agent runs | `magic "<topic>" [--out <path>]` | 9/10 | FTS5 query against the local `episodes` table (porter stemmer over `content_md, show, guests`), rank by BM25, write top-N to one markdown file with fixed-schema YAML frontmatter per episode | Brief verbatim: "chip-supply-chain.fly.dev workflow distilled to one command"; Killer Features #5; Lan persona's stated weekly ritual; no competitor ships a multi-episode bundler |
| 2 | Dispatcher decision trace before/after fetch | `episode get --explain <url>` | 8/10 | Run the cookie -> free -> paid chain in dry-run, record per-tier verdict (`skip: cookie_missing`, `skip: rss_lacks_transcript`, `match: dwarkesh_substack`) plus would-be cost, print the trace | Brief Source Priority table demands tier walking; absorbed #14 (cost-aware) only previews USD, not the dispatch logic |
| 3 | Local quote grep with speaker-tagged context | `episode quote "<phrase>" [-C N]` | 8/10 | FTS5 phrase query against `content_jsonl` segments, hydrate the matching segment plus N surrounding segments preserving `**Speaker** (MM:SS)` shape, print episode URL + deeplink timestamp | Trevin persona ("grep my podcast memory in 5 seconds"); absorbed #2 returns episode hits, not quote-level context |
| 4 | Cross-source content diff for one episode | `source compare <url>` | 7/10 | For an episode URL resolvable on >=2 sources, fetch all available sources, compare segment counts, total token counts, distinct-speaker counts, Levenshtein on speaker-label strings; emit a markdown table | Brief Validation Gates require per-source dogfood rows; absorbed list has no cross-source comparison |
| 5 | Bilingual side-by-side transcript for Xiaojun-class shows | `episode get <yt-url> --bilingual zh-Hans,en` | 7/10 | yt-dlp `--write-auto-subs --sub-langs zh-Hans,en --skip-download`, parse both VTTs, align by start timestamp (greedy nearest-neighbor), emit one markdown file with bilingual side-by-side body | Brief Six Target Shows row #6 (Xiaojun); `last30days-skill/scripts/lib/youtube_yt.py` is cited prior art |
| 6 | Per-service cookie freshness table | `auth status` | 6/10 | Read each `cookies-<service>.json`, check `expires_at` field + result of a HEAD against a known member URL; print one row per service | Brief Top Workflow #5 (auth login flow needs a status counterpart); user memory `reference_chrome_app_bound_encryption` shows cookies decay |
| 7 | Distinct-speaker corpus index | `speakers list [--show <name>]` | 6/10 | `SELECT speaker, COUNT(*) FROM segments GROUP BY speaker ORDER BY count DESC` over cached `content_jsonl` | Lan + Trevin both want "what do I have on Senra/Buffett"; mechanical SQL over local store |
| 8 | Spend pivot across shows + providers | `budget show --by-show [--since N]` | 6/10 | SQL pivot on `spend.jsonl` joined to `episodes` by `episode_url`; group by `show, provider, month` | Brief Build Priority #7 (spend tracking exists, pivot is the missing report) |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| `spend forecast <url-list>` | Collapses into Survivor #2 `--explain` which prints projected cost per tier | Survivor #2 |
| `magic --bundle-out <path>` with Hermes frontmatter | Not a separate feature; Survivor #1 ships fixed-schema frontmatter by default | Survivor #1 |
| `feeds digest --since 7d` | Overlaps absorbed #5 (`feeds sync`) + absorbed #2 (`episode search`). Only Lan persona benefits | absorbed #2 |
| `episode similar <url>` | Verifiability low without LLM judgment; BM25 "similar" is interpretable but adds noise | Survivor #7 (`speakers list`) |
| `speakers transcripts <name>` | Sibling of Survivor #7; thin script over list output | Survivor #7 |
| `cache reconcile` | Verifiability low; transcripts genuinely drift across sources | Survivor #4 (`source compare`) |
| `show coverage` | Partially absorbed by Survivor #6 (`auth status`) for cookie shows; Dwarkesh/Xiaojun don't need it | Survivor #6 |
| `magic --frontmatter-only` | Token-budgeting trick; head/yq over Survivor #1 covers it | Survivor #1 |
