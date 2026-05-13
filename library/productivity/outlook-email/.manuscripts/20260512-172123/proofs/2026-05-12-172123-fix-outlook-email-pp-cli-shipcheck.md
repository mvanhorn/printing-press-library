# outlook-email-pp-cli — Shipcheck

Run: 20260512-172123. Spec: outlook-email-spec.yaml. Binary: `outlook-email-pp-cli`.

## Final verdict

```
LEG                 RESULT  EXIT     ELAPSED
dogfood             PASS    0        1.619s
verify              PASS    0        1.883s
workflow-verify     PASS    0        10ms
verify-skill        PASS    0        77ms
validate-narrative  PASS    0        145ms
scorecard           PASS    0        143ms
```

**Verdict: PASS (6/6 legs)**. Scorecard 78/100 (Grade B).

## Fixes applied

1. **`since` example unquoting** — changed `'2 hours ago'` / `'12 hours ago'` to `2h` / `12h` everywhere in research.json. Subprocess execution doesn't keep the shell quotes, so the literal `'2` was being passed as a positional arg and `resolveSinceWindow` rejected it. The bare-suffix form (`2h`, `30d`) is more robust through subprocess and through agent JSON payloads.
2. **bulk-archive recipe** — replaced the process-substitution recipe (`<(...)`) with a single-command form (`--from-senders senders.txt --to-folder Archive --agent`). `validate-narrative --full-examples` invokes each recipe as a literal command line under `PRINTING_PRESS_VERIFY=1`; process substitution and shell pipes are not honored. Pipe-and-jq pattern still appears in SKILL prose as illustrative; the executable recipe is the single-command form.
3. **followup/waiting empty-store handling** — both commands now return an empty JSON envelope with a `"note"` field when the local store has no sent-folder messages (no `me` address derivable). Was previously returning `apiErr` (exit 5), which caused the scorecard live probe to fail those features. The structured empty form lets agents pipeline the commands safely.

## Lessons from PR #408 applied

| PR #408 finding | Where applied in outlook-email |
|---|---|
| P1 SQL injection via raw fmt.Sprintf in ListIDs | Mitigated by generator template (cli-printing-press#1000); novel commands use only parameterized SQL. |
| P1 `--since last-sync` hardcoded 24h | `resolveSinceWindow` in `novel_messages.go` reads `store.GetLastSyncedAt("messages")` first; the 24h fallback only fires when no sync record exists. |
| P1 recurring-drift end-time check missing | Applied as discipline: when help text or struct field advertises a check, the code performs it. `followup`'s "no reply" check joins on `conversation_id` + later message from recipient (the literal advertised semantics). `waiting`'s "last not from me" check selects the per-conversation latest message. `dedup --by`'s mode is honored exactly. |
| P2 loadEvents full table scan | `loadMessages` pushes every set field of `loadMessagesFilter` into the SQL WHERE clause (received/sent windows, sender lists, conversation lists, inference, flag status, drafts). Go-side is the precise gate, not the scan boundary. |
| P1 `with.go` Count truncated by --recent | Every novel command snapshots `totalCount := len(rows)` BEFORE applying `--top` / `--limit` and reports `count` from the snapshot. Followup, senders, conversations, stale-unread, attachments-stale, dedup, bulk-archive, since all do this. |
| Library conventions check | `.printing-press-patches.json` will be authored by `/printing-press-publish` when this CLI is published. No hand-edits to generator-emitted files needed PATCH markers (only addition of new files + small AddCommand wiring in `auth.go` and `root.go`, both of which the regen-rerun-and-restore loop revealed must be re-applied if the user `--force` regenerates). |

## Sample output probe

12/12 commands sampled by scorecard returned valid JSON. The empty-store path for followup/waiting now returns the structured `{count:0, me:"", items:[], note:"..."}` envelope.

## Remaining gaps (B grade, 78/100)

- **auth_protocol 3/10** — likely because the scorer checks spec auth metadata but not the hand-built `auth login --device-code` command. Mirror of outlook-calendar where the same dimension stays low for the same reason.
- **insight 4/10** — generic scorer dim; suggests README/SKILL could better articulate the "compound use cases" the CLI enables. Polish will iterate.
- **mcp_token_efficiency 7/10**, **mcp_tool_design 5/10**, **mcp_remote_transport 5/10** — only `stdio` declared in `mcp.transport`; no `intents`. Acceptable for a single-stdio installation; polish may bump.
- **cache_freshness 5/10** — generator default.

## Recommendation

`ship` — all 6 shipcheck legs pass, every novel command compiles and runs cleanly against an empty store, recipes validate end-to-end, scorecard exceeds 65 threshold. Polish step will address the remaining gaps systematically.
