# Cartesia Absorb Manifest

## Sources surveyed
- **cartesia-ai/cartesia-mcp** (official MCP, Python, 13★) — 7 tools: text_to_speech, list_voices, get_voice, clone_voice, infill, voice_change, localize_voice.
- **cartesia-ai/cartesia-python** (Stainless, 121★) — 50 SDK methods mirroring REST.
- **cartesia-ai/cartesia-js** (Stainless, 131★) — same surface in TS.
- **cartesia-ai/line** (voice-agent SDK, 100★) — runtime SDK, not a management surface; informs agent-runtime concepts.
- **ai-say-cli** (npm, TTS-only CLI) — 1 surface: TTS to speaker.
- **@livekit/agents-plugin-cartesia** (npm) — embeds Cartesia inside LiveKit agents.
- **@micdrop/cartesia**, **@restackio/integrations-cartesia** (npm) — framework adapters.
- **OpenAPI Stainless spec** (`storage.googleapis.com/stainless-sdk-openapi-specs/cartesia-noahlt/…yml`) — 50 endpoints across 35 paths.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|---------------------|-------------|
| 1 | List agents | python SDK / spec | `agents list` (+ `--limit`, `--json`, `--select`) | SQLite-backed, paginate offline, FTS on name/description |
| 2 | Get agent | python SDK | `agents get <id>` | local cache; `--watch` polls |
| 3 | Update agent | spec PATCH | `agents update <id>` (+ `--stdin` for JSON patch, `--dry-run`) | idempotent, agent-native exit codes |
| 4 | Delete agent | spec | `agents delete <id> [--force]` | confirms unless `--yes`; soft state reconciliation |
| 5 | List agent templates | spec | `templates list` (+ `--required-env-vars`, `--lang`) | filter by required envs / language hint |
| 6 | List phone numbers for agent | spec | `agents phone-numbers <id>` | join with local agent for context |
| 7 | List deployments per agent | spec | `deployments list --agent <id>` | sorted by created_at, with diff vs prior |
| 8 | Get deployment | spec | `deployments get <deployment-id>` | side-by-side prompt/voice diff vs previous deployment |
| 9 | List calls | spec | `calls list [--agent <id>] [--since 24h] [--status …]` | local FTS over transcripts |
| 10 | Get call | spec | `calls get <id> [--expand transcript]` | pretty turn-by-turn render |
| 11 | Download call audio | spec | `calls audio <id> [-o file.wav] [--play]` | safe defaults, optional system player |
| 12 | Tail call runtime logs | docs | `calls logs <id> [-f]` | follow mode like `kubectl logs -f` |
| 13 | Create LLM-judge metric | spec | `metrics create [--stdin]` | YAML/JSON input, dry-run |
| 14 | List metrics | spec | `metrics list` | local cache, FTS on criteria |
| 15 | Get metric | spec | `metrics get <id>` | shows attached agents |
| 16 | List metric results (paginated) | spec | `metrics results <id> [--since …] [--agent …]` | bounded; cursor handled |
| 17 | Export metric results CSV | spec | `metrics export <id> [-o file.csv]` | resume support; max 100k row guard |
| 18 | Attach metric to agent | spec | `metrics attach <metric> <agent>` | idempotent |
| 19 | Detach metric from agent | spec | `metrics detach <metric> <agent>` | idempotent |
| 20 | List voices | spec / mcp | `voices list [--lang …] [--gender …] [--owner me]` | SQLite-backed filters |
| 21 | Get voice | spec / mcp | `voices get <id>` | sample-play option |
| 22 | Clone voice from audio | spec / mcp | `voices clone <file> [--name] [--language] [--mode …]` | multipart upload streamed |
| 23 | Localize voice | spec / mcp | `voices localize <id> --language <code>` | dry-run + cost estimate |
| 24 | Update voice metadata | spec | `voices update <id>` | name/desc/gender |
| 25 | Delete voice | spec | `voices delete <id> [--yes]` | confirms |
| 26 | TTS bytes | spec / mcp / ai-say-cli | `tts bytes "<text>" --voice <id> [-o file.wav]` | pipe-friendly, stdin for text |
| 27 | TTS SSE | spec | `tts sse "<text>" --voice <id>` | streams to stdout |
| 28 | TTS WebSocket | docs | `tts ws "<text>" --voice <id>` (Cobra wrapper around persistent ws) | multiplexed contexts, --continue token |
| 29 | STT transcribe | spec | `stt transcribe <file>` | streams response |
| 30 | STT WebSocket (realtime) | docs | `stt ws --mic` or `stt ws <file>` | streams audio frames |
| 31 | Voice changer bytes | spec / mcp | `voice-changer bytes <file> --voice <id>` | swaps a voice in audio |
| 32 | Voice changer SSE | spec | `voice-changer sse <file> --voice <id>` | streamed output |
| 33 | Infill audio | spec / mcp | `infill <left.wav> <right.wav> "<text>"` | output between segments |
| 34 | Create fine-tune | spec | `fine-tunes create --dataset <id> [--base …]` | dry-run + cost estimate |
| 35 | List fine-tunes | spec | `fine-tunes list` | local cache |
| 36 | Get fine-tune | spec | `fine-tunes get <id>` | shows status, voice outputs |
| 37 | Delete fine-tune | spec | `fine-tunes delete <id>` | confirms |
| 38 | List voices from fine-tune | spec | `fine-tunes voices <id>` | local join |
| 39 | Create dataset | spec | `datasets create --name <n>` | idempotent name |
| 40 | List datasets | spec | `datasets list` | local cache |
| 41 | Get dataset | spec | `datasets get <id>` | file count + bytes |
| 42 | Update dataset | spec | `datasets update <id> --name …` | rename |
| 43 | Delete dataset | spec | `datasets delete <id>` | confirms |
| 44 | Upload file to dataset | spec | `datasets upload <id> <path>` | multipart, multi-file, --label |
| 45 | List dataset files | spec | `datasets files <id>` | local cache |
| 46 | Delete dataset file | spec | `datasets file-delete <id> <file-id>` | confirms |
| 47 | Create pronunciation dict | spec | `pronunciation-dicts create [--stdin]` | YAML/JSON entries |
| 48 | List pronunciation dicts | spec | `pronunciation-dicts list` | local cache |
| 49 | Get pronunciation dict | spec | `pronunciation-dicts get <id>` | show entries |
| 50 | Update pronunciation dict | spec | `pronunciation-dicts update <id>` | merge new entries |
| 51 | Delete pronunciation dict | spec | `pronunciation-dicts delete <id>` | confirms |
| 52 | Create access token | spec | `auth access-token --grant tts --ttl 60s` | grant-aware, JWT to stdout |
| 53 | API status / version | spec | `status` | parses `Cartesia-Version` from server |
| 54 | doctor (auth/connectivity/version) | best practice | `doctor` | health gate; `--json` |
| 55 | Sync everything to local store | (foundation) | `sync [--resource …] [--full]` | incremental cursor; per-resource opts |

(All 55 absorbed rows ship full. No stubs.)

## Transcendence (only possible with our approach)

| # | Feature | Command | Why Only We Can Do This | Score |
|---|---------|---------|--------------------------|-------|
| T1 | Deployment drift — diff prompt/voice/config between any two deployments of the same agent | `agents diff <agent-id> [--from <dep>] [--to <dep>]` | Needs both deployments resolved + a side-by-side renderer; API returns one deployment per call | 9/10 |
| T2 | Call grep — local full-text + regex over every transcript synced for an agent, with windowed context | `calls grep "<pattern>" [--agent <id>] [--turns 2] [--since 7d] [--json]` | Requires FTS5 over transcripts already pulled to disk; no single API call returns this | 10/10 |
| T3 | Metric trend — bucketed pass-rate / score-mean for any metric over time, sliced by deployment | `metrics trend <metric-id> --bucket day --agent <id> --since 30d` | Joins metric_results + deployment timeline locally; API only paginates raw results | 9/10 |
| T4 | Agent audit — single command that joins last 24h of calls + each call's metrics + deployment version, flags regressions | `agents audit <agent-id> [--since 24h] [--regression-threshold 0.1]` | Requires correlating four resources offline; compound queries the API can't answer in one shot | 10/10 |
| T5 | Voice library diff — show voices created/updated/deleted since last sync, with metadata diffs | `voices changelog [--since 7d]` | Needs historical local snapshots; API returns current state only | 7/10 |
| T6 | "What did I miss" for a deployed agent — every call + every flagged metric result since timestamp, in one feed | `agents since <agent-id> <when>` | Time-windowed aggregation across calls + metric_results; no single API endpoint provides this | 9/10 |
| T7 | Cost estimate before commit — given a fine-tune or localize request, estimate credits from local usage history | `usage estimate fine-tune --dataset <id>` / `usage estimate localize --voice <id>` | Combines `/usage/credits` history + planned operation type; agentic guess from local model | 6/10 |
| T8 | Bad-call hunter — surface the worst-scored calls across all agents in one window, with transcript snippets | `calls worst --metric <metric-id> --since 7d [--limit 20]` | Cross-agent ranking from local metric_results joined to call transcripts | 8/10 |
| T9 | Voice picker — match a free-text style description ("warm female, mid-pitch, Spanish") against local voices using FTS + filters | `voices find "<desc>" [--lang es] [--gender f]` | FTS5 + structured filters over locally cached voice catalog; no API search endpoint exists | 7/10 |
| T10 | Transcript SQL — let an agent compose arbitrary SELECT queries over the local store | `sql "SELECT * FROM calls WHERE …"` | Read-only SQL surface unique to local-store CLIs; gated to SELECT | 8/10 |

All 10 novel features score ≥6/10. None will ship as stubs.

## Group themes
- **Local state that compounds**: T1, T2, T3, T4, T5, T6, T8, T9, T10
- **Cost intelligence**: T7
