# Printing Press Retro — loopnet reprint, run 20260520-165006

Session: `/printing-press-reprint loopnet` → `/printing-press` reprint that added
`mcp.transport: [stdio, http]` and a `workflow_verify.yaml`. Outcome: shipped
(shipcheck PASS 6/6, scorecard 65). Below are systemic gaps for the maintainers.
The `printing-press-retro` sub-skill is not registered in this environment, so
this is a manual capture. Filing as a GitHub issue is left to the operator.

## Finding 1 — Sniffed-CLI auth model mismatch (generator / detection)

`loopnet` spec declares `auth.type: none`, but live data fetches require Akamai
**browser-clearance cookies**. Symptoms:
- `probe-reachability` reported `needs_clearance_cookie: false` — it probes the
  homepage (Surf clears it); search/detail data pages do **not** clear with Surf.
- `dogfood` `browser_session_check` → `required: false`.
- The spec carries no browser-clearance marker.
- Yet the generated CLI ships an `auth refresh`/`set`/`status` command for
  clearance cookies and `research.json.auth_narrative` describes them as required.

The machine's auth detection and the CLI's real auth model disagree. **Suggest:**
when a sniffed CLI's runtime is clearance-gated (or it ships a clearance `auth`
command), mark browser-clearance auth required in the spec/traffic-analysis so
probe, dogfood, scorecard, SKILL prose, and Phase 5 stay consistent — and so
`probe-reachability` distinguishes "homepage clears" from "data pages clear".

## Finding 2 — workflow-verify classifyError uses error-text substrings, not exit codes

`classifyError` (internal/pipeline/workflow_verify.go) classifies a failed step
`blocked-auth` only if stdout/stderr contains "403"/"forbidden"/"unauthorized"/
"authentication required"; otherwise `fail-cli-bug`. The loopnet `sync` step
exited with the framework's typed `authErr` code and an honest message ("Akamai
bot-challenge page — clearance cookies missing"), but lacked those literal
tokens → misclassified `fail-cli-bug` → false `workflow-fail`. **Workaround
applied this run:** edited the CLI's challenge message to include "HTTP 403 …
Authentication required". **Suggest:** `classifyError` should consult the
command's typed exit code (the framework already defines a typed auth exit code)
rather than substring-matching free text.

## Finding 3 — Doc examples generated from spec params, not hand-customized commands

The generator emitted README/SKILL `inventory` examples as
`inventory <property_type> <location> <listing_type>` (3 positionals, from the
spec resource) and `inventory mock-value mock-value mock-value`. The CLI's
hand-written `promoted_inventory.go` overrides the signature to
`inventory <location>` + `--type`/`--listing` flags. verify-skill flagged it a
"likely false positive"; it is a real mismatch. **Suggest:** when a promoted
command is hand-customized, generate doc examples from the built binary's
`--help`, or reconcile spec-derived examples against the binary in the audit.

## Finding 4 — Generator never emits workflow_verify.yaml

`workflow-verify` with no manifest reports "no workflow manifest found,
skipping" and a vacuous `workflow-pass` (the prior loopnet run shipped that
way). **Suggest:** emit a starter `workflow_verify.yaml` from the spec's
primary workflow / promoted commands so workflow-verify is meaningful by default.

## Finding 5 — Reprint port hazard: hand-customized generator-named files (process)

The prior CLI hand-rewrote 5 files sharing generator-emitted filenames
(`sync.go`, `promoted_inventory.go`, `promoted_property.go`,
`channel_workflow.go`), each headed "Hand-written … replaces the generated
generic …". A port that carries only "files absent from fresh scaffolding"
silently keeps the generic versions of these (they exist in both dirs).
**Suggest:** reprint/port guidance should diff ALL common files and detect the
"Hand-written … replaces the generated generic" header marker — not just port
absent files. Caught and corrected this run via a full common-file diff.

## Environment note
`printing-press-polish`, `printing-press-output-review`, and
`printing-press-retro` sub-skills were not registered in this Claude Code
environment — only `printing-press`, `printing-press-reprint`, and
`printing-press-import`. Phases 4.85, 5.5, and 6-retro fell back to manual
equivalents. Worth checking the plugin's skill registration / packaging.
