# Phase 4.8 and 4.9: SKILL and README review

Reviewed inline by the printing agent (no separate reviewer subagent; scope set by operator direction).

## SKILL.md (Phase 4.8)
1. Trigger phrases match the commands: PASS.
2. Recipes run: PASS (validate-narrative --strict --full-examples, 9 narrative commands).
3. Novel-feature descriptions match commands: PASS. `cited --help` says the same as the Unique Capabilities line; the help also states the credit cost per engine.
4. Stub or gated disclosure: PASS. No stubs. Paid calls are disclosed (per-engine credit cost in `cited` help and in the quickstart comments).
5. Auth: PASS. `CLORO_API_KEY`, key page and free plan in the auth description.
6. Anti-triggers: PASS, three concrete boundaries.
7. Marketing-copy smell: PASS. No "comprehensive", "seamless", "powerful".

## README.md (Phase 4.9)
- Every command path in Quick Start, Recipes and Unique Features resolves (verify-skill PASS, validate-narrative PASS).
- Fixed: the Quick Start comment for `cited` now states the cost of the default run (26 credits for four engines).
- Fixed: a troubleshooting line that verify-skill read as a `credits --engines` call.
- Open (template text, not changed in Go source): the generated Go comments and the root `--help` Short line carry em dashes from the generator templates.

Verdict: PASS
