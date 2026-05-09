# Build Log — Substack CLI

## Run
- Run ID: 20260509-103230
- Spec: `research/substack-spec.yaml` (agent-curated internal YAML, ~620 lines)
- Working dir: `working/substack-pp-cli/`

## What was built

### Generator (Phase 2)
- 67 Cobra command files emitted under `internal/cli/`
- 16 resource groups: categories, profiles, notes, comments, inbox, discover, settings, posts, drafts, sections, tags, images, recommendations, subs, feed
- All 8 quality gates passed: go mod tidy, govulncheck, go vet, go build, binary build, --help, version, doctor
- MCP build artifact: `build/substack-pp-mcp-darwin-arm64.mcpb`

### Phase 3 hand-built transcendence (12 new files)
- `internal/cli/growth.go` + 3 subcommands (attribution, best-time, pod)
- `internal/cli/engage.go` + 4 subcommands (reciprocity, like, restack, restack-with-comment)
- `internal/cli/voice.go` + voice fingerprint
- `internal/cli/recs.go` + recommendations find-partners
- `internal/cli/discover_patterns.go` (extends discover)
- `internal/cli/notes_schedule.go` + cadence guard (typed exit 2)
- `internal/cli/notes_new.go` (Markdown→ProseMirror convenience over `notes create`)
- `internal/store/store.go` extensions: 5 new tables (engagements, notes_queue, analytics_snapshots, reach_windows, inspiration_notes) with idempotent CREATE
- `internal/notebuilder/notebuilder.go` + 8 unit tests (all passing)

### Phase 3 acceptance tests (12/12 PASS)
- attribution empty + seeded
- reciprocity (filtered + unfiltered)
- pod matrix
- best-time (top cell ranked)
- discover patterns (mechanical extraction)
- find-partners (empty + seeded)
- voice fingerprint (empty + seeded with 7 metrics)
- notes schedule (success + cadence violation exit 2)
- engage like (print-curl + verify-env short-circuit)
- auth login --chrome verify-env short-circuit
- --help surface (growth, engage, voice, recs all visible)
- internal/notebuilder unit tests

## Intentionally deferred

- **Engage write endpoints (like/restack/restack-with-comment)** ship with the print-curl fallback because no community wrapper has reverse-engineered them. `--send` exists for the day they're confirmed via DevTools capture; `cliutil.IsVerifyEnv()` short-circuits them in mock mode.
- **Substack Chat / DMs** — endpoints unmapped (NHagar PR #19 stale). Out of scope; flagged for a follow-up run with browser-sniff.
- **LinkedIn / X cross-post bridges** — require external auth not provisioned this run.
- **AI Note generation `notes generate`** — Tier B; not built. The brief promised an honest stub printing the prompt template; that work didn't fit the session.
- **Multi-client / ghostwriter mode** — Tier B; not built.

## Surface to fix in a follow-up

- `growth pod` matrix attribution is best-effort (engagements schema lacks `actor_handle` column). Acceptance test passes (matrix shape valid, members listed) but cell semantics are placeholder until pod-tracking sync writes actor_handle.
- `discover patterns` shares the namespace with the existing `discover` leaf command (publication search). `discover patterns --help` works; the parent shows the leaf's short doc. Cleaner separation needs a generator change.
- Auth Chrome cookie import uses the existing shell-out path the generator emits (no Go-native `kooky` dep added).

## Generator limitations encountered (retro candidates)
- Internal YAML reserved-name list includes `profile` and `analytics`; needed to rename. The reserved-name check is preventive and good — kept us from name collisions later.
- Aliases must be lowercase kebab-case; underscore wire-name aliases (e.g., `[note_id]`) are rejected. Stripping them was the right call (kebab is the convention).
- `endpoint_template_vars` works cleanly with per-resource `base_url` overrides for the per-publication surface.
