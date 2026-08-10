# Phase 4.8 (Agentic SKILL Review) + 4.9 (README/SKILL/AGENTS Audit)

Two parallel read-only reviewers against the built CLI + research.json.

## Fixes applied
- research.json novel_features "page get" description: removed the unverifiable
  "warning when the target differs from the local doc's version" claim (page
  get fetches the versioned tree live; no local-doc drift warning exists).
  Synced to SKILL.md/README.md via dogfood.
- research.json headline: softened unqualified "Every Q-SYS product spec" to
  "Q-SYS product specs..." (coverage makes extraction partial by design).
- research.json when_to_use + coverage why_it_matters: "once synced"/"after a
  sync" -> "once harvested"/"after a harvest" (harvest builds the corpus, not
  sync).
- SKILL.md: removed credentials.toml / auth-sidecar / "first auth write"
  boilerplate (auth.type none — no credentials exist); honest data-dir
  description; "credential-location warnings" -> "path warnings"; "credentials
  left under the former root" -> "corpus left".
- SKILL.md + README.md: `networking mock-value` placeholder examples replaced
  with real topic `QoS.htm`; dropped the wrong `--select id,name,status` field
  list.
- README.md: dropped "per-command header overrides take precedence" (no CLI
  surface for it).

## Clean areas (verified PASS by reviewers)
- Trigger phrases map to real commands.
- Unique Capabilities == novel_features_built (8) exactly; product compare/sql
  correctly not claimed as headline.
- All commands/subcommands/flags/exit codes in docs resolve against the binary.
- No CRUD/auth/rate-limit claims the CLI doesn't back; no-auth handled
  correctly; brand "Q-SYS" canonical; anti-triggers present; AGENTS.md
  accurate (agent-context schema v4, which semantics, teach.log/feedback.jsonl
  paths).
