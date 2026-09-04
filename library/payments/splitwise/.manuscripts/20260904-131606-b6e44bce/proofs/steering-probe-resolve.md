# sw-resolve steering probe — 4.31.7 (run 20260904-131606-b6e44bce), 2026-09-04

Prompt (verbatim CDR 5.5): What's the friend id for "Michael"?
Subject: MCP server + CLI built from the working tree at scratch commit cb68d5e (P5; BEFORE the resolve Long/scope-redirect), store = sqlite3 .backup copy of the operator store under a scratch HOME; MCP env adds PRINTING_PRESS_DOGFOOD=1 (write paths refuse). No names or ids recorded.

## A. Real-world environment (plain claude -p, installed pp-splitwise skill + CLI on PATH visible), 12 trials
MCP resolve called 4/12; 8/12 → Skill(pp-splitwise) → Bash → mostly 'get-friends --agent --select …| grep -i michael' (2 of those ran 'resolve … --type friend --agent' via Bash first, then re-checked with get-friends). ToolSearch was used to load the deferred MCP tool in the MCP-path trials.

## B. Harness-like, allowlist variant (--strict-mcp-config --allowedTools mcp__splitwise__* + AQL deny list), 8 trials
resolve first call 8/8; redundant verification call after it 6/8 (get-friend_get ×4, sql ×2). Confound: the allowlist itself steers toward MCP tools.

## C. AQL-runner-faithful (deny list only: Bash Skill WebSearch WebFetch ToolSearch Task Read Edit Write Glob Grep NotebookEdit TodoWrite; --strict-mcp-config; --dangerously-skip-permissions; --max-turns 8), 20 trials — THE '4.31.7 bare' DATAPOINT
    trial 1 [success]: mcp__splitwise__resolve mcp__splitwise__get-friend_get 
    trial 2 [success]: mcp__splitwise__resolve mcp__splitwise__sql 
    trial 3 [success]: mcp__splitwise__resolve mcp__splitwise__get-friends_list 
    trial 4 [success]: mcp__splitwise__resolve mcp__splitwise__get-friend_get 
    trial 5 [success]: mcp__splitwise__resolve mcp__splitwise__sql 
    trial 6 [success]: mcp__splitwise__resolve mcp__splitwise__get-friend_get 
    trial 7 [success]: mcp__splitwise__resolve mcp__splitwise__get-friend_get 
    trial 8 [success]: mcp__splitwise__resolve mcp__splitwise__sql 
    trial 9 [success]: mcp__splitwise__resolve mcp__splitwise__sql 
    trial 10 [success]: mcp__splitwise__resolve mcp__splitwise__get-friend_get 
    trial 11 [success]: mcp__splitwise__resolve mcp__splitwise__get-friend_get 
    trial 12 [success]: mcp__splitwise__resolve mcp__splitwise__get-friend_get 
    trial 13 [success]: mcp__splitwise__resolve mcp__splitwise__get-friend_get 
    trial 14 [success]: mcp__splitwise__resolve mcp__splitwise__get-friend_get 
    trial 15 [success]: mcp__splitwise__resolve mcp__splitwise__sql 
    trial 16 [success]: mcp__splitwise__resolve mcp__splitwise__get-friend_get 
    trial 17 [success]: mcp__splitwise__resolve mcp__splitwise__get-friend_get 
    trial 18 [success]: mcp__splitwise__resolve 
    trial 19 [success]: mcp__splitwise__resolve mcp__splitwise__get-friends_list 
    trial 20 [success]: mcp__splitwise__resolve mcp__splitwise__sql 

first_call_resolve=20/20; any_resolve=20/20; redundant verification 19/20 (get-friend_get 11, sql 6, get-friends_list 2); straight answer 1/20.
Baseline for comparison: AQL run 7600 (2.0.0 build, reps=7): sw-resolve e2e 3/7, isolation 0/7 (AQL's runner, same deny list).
Caveats: n=20, one store, single prompt; AQL's aql_ref binds the same name; AQL certify runs measure the whole surface at reps=7 and per-prompt rates are not comparable across reps.

## Refinements agreed with `aql-splitwise` (before the Long lands)
- **Regime mapping:** each probe trial is a fresh single-prompt session → this maps to AQL's *isolation* limb (one fresh
  session per prompt), NOT e2e (one session threaded across 17 prompts with --resume, so `resolve` arrives after ~16 turns
  of prior context). Like-for-like comparison is therefore **20/20 vs isolation 0/7** (one-sided Fisher p ≈ 1e-6), not vs
  e2e 3/7.
- **Prediction banked now:** if 4.31.7 genuinely fixes this, the certify ISOLATION limb should move sharply and e2e should
  move less (description is a weaker lever against 16 turns of priming). e.g. isolation 0/7 → 6/7 with e2e 3/7 → 4/7 is
  consistent with a real fix, not a partial one.
- **Residual deviations from the harness in datapoint C:** `--max-turns 8` (runner sets no cap; a 2–3 turn task should not
  bind) and MCP env `PRINTING_PRESS_DOGFOOD=1` (runner requires it unset; read paths unaffected, set here so write tools
  refuse under --dangerously-skip-permissions).
- **Real success metric for the "ids are authoritative" Long is the redundant-verification rate** (6/8 allowlist → 19/20
  runner-faithful — more entrenched under faithful flags), which AQL's membership test cannot see. Datapoint D (post-Long)
  must report both first-call and redundant-verification counts.

## D. AQL-runner-faithful, AFTER the resolve scope-redirect Long (scratch commit 49d69af / P7), 20 trials — same flags as C
    trial 1 [success]: mcp__splitwise__resolve 
    trial 2 [success]: mcp__splitwise__resolve 
    trial 3 [success]: mcp__splitwise__resolve 
    trial 4 [success]: mcp__splitwise__resolve 
    trial 5 [success]: mcp__splitwise__resolve 
    trial 6 [success]: mcp__splitwise__resolve 
    trial 7 [success]: mcp__splitwise__resolve 
    trial 8 [success]: mcp__splitwise__resolve 
    trial 9 [success]: mcp__splitwise__resolve 
    trial 10 [success]: mcp__splitwise__resolve 
    trial 11 [success]: mcp__splitwise__resolve 
    trial 12 [success]: mcp__splitwise__resolve 
    trial 13 [success]: mcp__splitwise__resolve 
    trial 14 [success]: mcp__splitwise__resolve 
    trial 15 [success]: mcp__splitwise__resolve 
    trial 16 [success]: mcp__splitwise__resolve 
    trial 17 [success]: mcp__splitwise__resolve 
    trial 18 [success]: mcp__splitwise__resolve 
    trial 19 [success]: mcp__splitwise__resolve 
    trial 20 [success]: mcp__splitwise__resolve 

first_call_resolve=20/20; any_resolve=20/20; **redundant verification 0/20** (was 19/20 in C); every trial answered straight from resolve.
Reading: the Long's "returned ids are authoritative" line removed the re-check behaviour entirely under harness-faithful conditions; first-call steering was already 20/20 on 4.31.7 bare. Same caveats as C (n=20, one store, one prompt, --max-turns 8, PRINTING_PRESS_DOGFOOD=1 on the MCP env).

## RETRACTION (from `aql-splitwise`, after certify run 7604)
AQL's runner sends the agent the record's `pass:` criteria text, not the fenced prompt (`_probe_text()` → `pass_prose or cli`;
the prose block is never parsed). For sw-resolve the agent received only "matching id(s) for the name, as a top-level array".
Controlled A/B, runner flags, stamped 2.0.0 binaries: real question → resolve 5/5; `pass:` fragment → 0/5 with no tool call.
Therefore isolation 0/7 on 2.0.0 and 4.31.7, and baseline 7600's steering RED, are instrument artifacts — not a resolve defect.
Datapoints A–D above stand as the valid evidence (real prompt): 20/20 first-call, 0/20 redundant re-check after the Long.
Comparison "20/20 vs 0/7" in C is void (different stimuli). Filed as an AQL engine issue by `aql-splitwise`.
