# AnkiWeb Novel-Features Brainstorm (subagent audit trail)

## Customer model

**Maya — language learner (Spanish/Japanese), self-directed**
- Today: browses the slow shared-decks SPA, opens decks one-by-one to check note counts and audio; can't compare candidates.
- Weekly ritual: searches a topic, eyeballs upvote/downvote, downloads one or two.
- Frustration: approval rate buried, audio coverage not filterable, no offline re-run of last week's search to see what's new.

**Devin — med student curating an anatomy/pharmacology study set**
- Today: hunts the "best" deck among near-duplicates, guesses freshness from modified dates, keeps a shortlist in a Doc.
- Weekly ritual: re-checks for higher-rated/fresher decks; cross-references note counts.
- Frustration: can't rank by a single quality signal, can't tell stale from maintained, no scriptable batch view.

**Priya — Anki power user who has published shared decks**
- Today: logs into ankiweb.net to check synced decks/card counts and download counts on her shared decks.
- Weekly ritual: glances at cloud decks/stats; notes download gains.
- Frustration: no terminal/scriptable view of her synced decks; can't track download-count drift over time.

## Candidates (pre-cut)
(14 candidates generated — see survivors/kills below. Sources: persona-driven, service-specific content patterns (ratings/approval, audio coverage, freshness, note counts), cross-entity local queries, user-briefing.)

## Survivors and kills

### Survivors (transcendence table)
| # | Feature | Command | Score | Persona | Buildability proof |
|---|---------|---------|-------|---------|--------------------|
| 1 | Approval-rate ranking | `shared rank <term>` | 8/10 | Maya, Devin | upvotes/(upvotes+downvotes) with min-vote floor over shared_deck rows; sort. No external deps. |
| 2 | Audio/image coverage filter | `shared search --has-audio` / `--has-images` | 7/10 | Maya | Filter on audio/images count fields present in list-decks; mechanical predicate. |
| 3 | Side-by-side comparison | `compare <id> <id> [...]` | 7/10 | Devin, Maya | Join multiple cached shared_deck rows into one table; pure local SQLite read. |
| 4 | Freshness ranking | `shared fresh <term>` / `--since` | 6/10 | Devin | Order shared_deck rows by modified_unix; local ordering. |
| 5 | New-deck drift watch | `watch <term> --since-last-sync` | 6/10 | Maya, Devin | Diff current list-decks result vs last SQLite snapshot; list new/changed. Degrades w/o download. |
| 6 | Owned-deck download drift | `drift` | 6/10 | Priya | Snapshot owned shared-deck download counts (auth) to SQLite; cross-sync deltas. |
| 7 | Discovery briefing | `brief <term>` | 6/10 | Maya, Devin | Compose top-N approval + audio coverage % + freshest + new-since-sync into one --json digest. |

### Killed candidates
| Feature | Kill reason | Closest sibling |
|---------|-------------|-----------------|
| `decks stats` | Auth stat fields unconfirmed; low verifiability | `drift` |
| `dupes <term>` | Fuzzy title grouping, not deterministically verifiable | `compare` |
| `shared reviews <id>` | Sub-field of already-absorbed `shared info` | `shared info` |
| `shortlist add/list/rm` | Scope creep toward stateful app | `compare` |
| `recommend <term>` | Collapses to `shared rank \| head -1` | `shared rank` |
| `categories` | Not grounded in a workflow; not confirmed in /svc surface | `shared search --has-audio` |
| `gaps` | Requires semantic topic definition; LLM-adjacent | `watch` |
