# Phase 14 semantic SKILL review

Reviewed SKILL.md, README.md, run research.json and approved absorb-manifest.md against root help and help for all five unique capabilities, catalog find, and edgar facts. No live HTTP calls made.

## Findings

1. **Command discovery coverage — error — SKILL.md:73.** Command Reference omits the shipped catalog, dataset, resource, edgar and custom sync command families; Direct Use later tells agents to select from this reference. Add concise entries for those actual command paths, including all five EDGAR operations and snapshot creation/listing. README Commands has the same omission.

2. **Response shape — error — SKILL.md:180-189.** The text says API/store commands always wrap output and tells agents to parse `.results`. Ordinary `--json` SEC company/facts responses are raw upstream objects; provenance wrapping is selected by `--agent`. Restrict the envelope promise to agent mode and explain that commands with existing envelopes preserve their own shape.

3. **Local persistence contract — error — SKILL.md:218.** “Reads in ... auto mode are written to the SQLite store” overstates custom behavior: catalog find persists a snapshot only with `--save`, and direct SEC commands do not persist filing/facts responses to the catalog SQLite store. Qualify this as generated-endpoint behavior and document explicit catalog `--save`/sync persistence. README Local data makes the same blanket assertion.

## Seven-check results

- Trigger phrases: PASS; each refers to implemented catalog, download or EDGAR behavior.
- Verified-set alignment: PASS; exactly overlap, changes, intake lock, intake audit and coverage match novel_features_built.
- Novel descriptions versus command help: PASS; five help scopes match the unique-capability descriptions and sibling redirects.
- Stub/gated disclosure: no stubs found. Saved-data requirement is evident in descriptions; recommend one concrete `catalog find ... --save ...` or `sync` recipe before local commands.
- Auth narrative: substantively PASS; California anonymous, federal key/demo scope, SEC_USER_AGENT no-key story matches implementation. Add exact DATA_GOV_API_KEY name for actionable federal setup.
- Recipe output claims: PASS for the catalog projection and Assets-concept recipes. The generic envelope guidance has the error above.
- Marketing smell: PASS; capability text is concrete and avoids unsupported ranking or quality claims.

These are documentation findings only. No code or documentation was changed by this reviewer. Source-driven narrative changes must be made in run research.json and synchronized; structural command-reference/envelope clarifications may use their local doc fix path.
