# Novel Features Brainstorm — cmux-pp-cli

(Full subagent output preserved verbatim; survivors flow into the absorb manifest.)

## Customer model

### Persona 1: Damien — "The orchestrator-runner"
**Today.** Runs `/ecosystem-manager` and `/cto-agent` which poll every cmux pane via `capture-pane` each tick. Each poll burns context tokens reading screens he mostly does not need. 6–12 workspaces open across Tuck, tuck-fs, Margin, CM, ecosystem. Squints at title icons to figure out which pane is awaiting input.

**Weekly ritual.** Multiple times per day, "where do things stand across my terminals?" The manager walks every pane, captures screen, classifies state, surfaces NEEDS-YOU items. Answers menus, dispatches mechanical next steps.

**Frustration.** Polling is the ToC constraint. The orchestrator does not know *when* a pane flipped to Needs-input, only that it is now. No timeline → no "stuck > 30 min" alert. Search returns workspaces but not surface or snippet. Notify is sidebar-only; nothing reaches macOS or Slack.

### Persona 2: ecosystem-manager / cto-agent subagent — "The polling skill"
**Today.** Reads `snapshot_panes.py` output every tick. Walks panes, diffs, decides which transitioned INTO `Needs input`. Capturing pane content per tick is expensive; most ticks have zero transitions.

**Weekly ritual.** Fires whenever Damien invokes the skill or on `/loop`. Reads session JSON, computes diff, writes append-only JSONL logs. Returns NEEDS-YOU cards.

**Frustration.** No event source — every tick is a full scan even when nothing changed. Cannot answer "how long has Tuck been awaiting?" because it never persists snapshots. Cross-pane content search would require capturing every pane every tick; instead it skips.

### Persona 3: Matt — "The multi-agent power user"
**Today.** Runs 3–5 Claude Code sessions in parallel across forks/worktrees. Uses `find-window --content` to locate which pane mentioned a function name. Result: workspace match, no surface, no snippet — has to alt-tab.

**Weekly ritual.** Spawns parallel sessions for feature work, debugging, review. Drops into different workspaces by keyboard.

**Frustration.** Title-icon decoding is folklore. No alert when a long-running build finishes. `cmux notify` lands in cmux's own sidebar — invisible while he is in another app.

## Candidates (pre-cut)
1. (b,e) `search "<q>" [--switch]` — FTS over surfaces.title, notifications.body, pane_content_samples.text, workspaces.title. **Keep.**
2. (b,e) `watch [--sink ...] [--source notifications|fsnotify]` — long-running notification poll + sinks. **Keep.**
3. (b,c) `status timeline [--workspace W] [--since 1h]` — query status_snapshots. **Keep.**
4. (b,c) `status stuck --over 30m` — latest snapshot per (workspace, key) where value=Needs input and ts older than N. **Keep.**
5. (b,e) `status awaiting --all` — normalized state column. **Keep.**
6. (b,e) `alert add/list` — declarative state-transition rules → sinks. **Keep.**
7. (b) `status icons --title` — pure local title decoder. **Keep (in absorb).**
8. (b,c) `status changes --since 1h` — recently-flipped workspaces. **Keep.**
9. (b,e) `panes sample` — populate FTS table. **Keep (plumbing for #1).**
10. (c) `status heat --bucket 1h` — heatmap. **Cut (no weekly use).**
11. (b) `notifications inbox --unread` — thin wrapper. **Cut (absorbed row #8 already).**
12. (b,e) MCP-exposed read tools. **Cut (generator default, not a feature).**
13. (e) `alert send --to slack` — one-site wrapper. **Cut (overlaps #6 sink).**
14. (b) Standalone fsnotify daemon. **Cut (collapsed into #2 as flag).**
15. (c) `workspaces card <ref>` — local cross-entity join. **Keep.**
16. (b,e) `alert --auto-clear` — side-effect overlap. **Cut.**

## Survivors and kills

### Survivors (transcendence rows)
1. `search "<query>" [--switch]` — score 9/10 — FTS5 over surfaces.title, notifications.body, pane_content_samples.text, workspaces.title; `--switch` calls `surface.focus`. Persona: Damien + Matt. Persona served, evidence in brief Top Workflow #1 + User Vision quote.
2. `watch [--sink stdout|file|exec|slack|webhook] [--source notifications|fsnotify]` — score 9/10 — long-running notification poll or fsnotify on session JSON; emits JSONL per event. Persona: ecosystem-manager. Evidence in Top Workflow #2.
3. `status timeline [--workspace W] [--since 1h]` — score 8/10 — query local `status_snapshots`. Persona: Damien. Evidence in Top Workflow #3.
4. `status stuck [--over 30m]` — score 8/10 — latest snapshot per (workspace, key) past threshold. Persona: Damien. Evidence in Top Workflow #4.
5. `status awaiting [--all]` — score 7/10 — normalized state column. Persona: Damien + manager. Evidence in Top Workflow #4.
6. `alert add/list` — score 7/10 — declarative rules + sinks. Persona: Damien + Matt. Evidence in Top Workflow #5 + User Vision.
7. `status changes --since 1h` — score 7/10 — recently-flipped workspaces. Persona: manager. Evidence in Top Workflow #3.
8. `workspaces card <ref>` — score 6/10 — cross-entity join. Persona: Damien. Evidence in Build Priorities.

### Killed candidates
| Feature | Kill reason | Closest surviving sibling |
|---|---|---|
| `status heat --bucket 1h` | Speculative weekly use | #7 status changes |
| `notifications inbox --unread` | Thin wrapper over `cmux list-notifications` + filter | Absorbed row #8 |
| MCP-exposed read tools as a feature | Generator default, not a differentiator | Cobratree walker |
| `alert send --to slack` | One-site thin wrapper; overlaps sink machinery | #6 sink |
| Standalone fsnotify daemon | Persistent background process is app-shaped | #2 watch --source fsnotify |
| `alert --auto-clear` | Side-effect overlap; weakens print-by-default | #6 plus notifications write side |
| `panes sample` as novel | Store-population plumbing (carve-out) | Backs #1 search |
| `status icons --title` as novel | Atomic helper, already absorbed | Absorbed row #15 |
