---
title: Printing Press Machine Upgrades — Systemic Issues Surfaced by SF360 Verification
type: feat
status: advisory
date: 2026-04-24
origin: /tmp/sf360-findings/README.md
companion: 2026-04-24-001-fix-sf360-live-verify-findings-plan.md
target_repo: https://github.com/mvanhorn/cli-printing-press
---

# Printing Press Machine Upgrades

**Advisory artifact.** This file describes systemic bugs and gaps in the CLI Printing Press **generator** (not in `salesforce-headless-360-pp-cli` or any other specific generated CLI). Each upgrade listed here would likely improve **every** CLI the Printing Press produces, not just the Salesforce one.

This is **not for implementation in this repo**. It's a prescription to be taken back to [`mvanhorn/cli-printing-press`](https://github.com/mvanhorn/cli-printing-press) as a planning input for the next generator iteration.

## How findings were filtered

The [live-verification findings log](/tmp/sf360-findings/README.md) contains 20 findings (F-001 through F-020). A finding earned a slot in this artifact only if it met ONE of three bars:

1. **Codegen pattern bug.** The generator emits the same broken pattern regardless of input API. Example: `metadata/fields/<SObject>.Field.field-meta.xml` flat layout is non-SFDX — any SF-targeting CLI would hit it. The pattern "emit metadata at a flat path" would likely fail similarly for Terraform, K8s, or any other platform with a structured source tree.
2. **Template gap.** The generator's plan/doc/script templates are missing a capability that any generated CLI needs. Example: runbook plans assume pre-existing org data; no seed helper exists in the template.
3. **Missing validation step.** The generator ships without validating its own output. Example: no smoke-test asserts that commands shown in README/SKILL.md actually parse when executed.

Findings unique to Salesforce (Task/Event Activity-object limitations; User-lookup cascade rules; sandbox password snapshot) were **filtered out**. Those are in the companion local-fix plan.

## Upgrade list

### U1 — Platform-aware metadata/resource layout emitter

**Rationale:** F-007, F-010.

**What the generator does today:** for Salesforce metadata, it emits CustomField files at `metadata/fields/<SObject>.<Field>.field-meta.xml` (flat) and places Custom Metadata Types at `metadata/<CMDT_Name>/` (root). Neither matches the SFDX source format that modern `sf project deploy start` expects.

**What the generator should do:**
1. Maintain a per-platform layout spec. For Salesforce SFDX source format: fields at `objects/<SObject>/fields/<Field>.field-meta.xml`; CMDTs at `objects/<Name>__mdt/`; custom objects at `objects/<Name>__c/`; permission sets at `permissionsets/<Name>.permissionset-meta.xml`.
2. Emit `sfdx-project.json` at the metadata root so `sf project deploy start --source-dir metadata` is valid from a fresh clone.
3. Include a per-platform emitter contract: for each resource type, where does the file go + what's the expected suffix.
4. Apply equivalents for other platforms: Terraform (per-provider canonical module paths); K8s (`kustomize`/`helm` layout); `dbt` (models/ tree); etc.

**How to validate:**
- Smoke test: from a fresh clone of a generated CLI that ships metadata, the platform's own tooling accepts the tree without preprocessing.
- Regression test: a corpus of "known-good" metadata trees per platform; generator output must diff-match canonical structure.

**Why it's systemic:** any CLI the generator produces for a platform with a prescriptive source tree will hit this if the generator picks the wrong layout. Salesforce is not special.

---

### U2 — Schema-compliant declarations per resource subtype

**Rationale:** F-009, F-012, F-013, F-014.

**What the generator does today:** emits schema fragments as if Custom Metadata Types, Custom Objects, Custom Fields, and Permission Sets share a single superset of valid elements. They don't.

**Specific bugs observed (all in SF metadata but representative of the class):**
- `<deploymentStatus>Deployed</deploymentStatus>` — valid for `CustomObject`, rejected on `CustomMetadataType` (F-009).
- `<defaultValue>pending</defaultValue>` at the top level of a restricted picklist CustomField — treated as a formula expression and rejected when the picklist's `valueSetDefinition` already declares `<default>true</default>` on the same value (F-012).
- `<required>true</required>` on a LongTextArea CustomField — platform-rejected (F-013).
- `<fieldPermissions>` declarations for required fields in a PermissionSet — platform-rejected as redundant (F-014).

**What the generator should do:**
1. Validate emitted metadata against the platform's schema **before writing the file**, per resource subtype. For Salesforce: consult the Metadata API element support matrix (e.g., no `deploymentStatus` on CMDT, no `required` on LongTextArea).
2. Ship a per-platform pre-flight validator (runnable as `printing-press validate <generated-dir>`) that exits non-zero on any known violation.
3. Catch the "emit `<required>true</required>` on a field whose type rejects it" class of bug generally — maintain a per-platform "field-type ↔ allowed-constraint" matrix.
4. For Salesforce specifically: check all CMDT files against CustomMetadataType schema (not CustomObject schema); check LongTextArea required rule; check required-field FLS rule.

**How to validate:**
- Smoke test: a generated CLI's metadata tree passes the platform's own deploy/validate step (e.g., `sf project deploy validate` on SF, `terraform plan` on TF) on a fresh empty target.

**Why it's systemic:** any platform with per-subtype schema rules (Salesforce, Terraform, K8s CRDs, OpenAPI) has this failure mode.

---

### U3 — Flag schema consistency

**Rationale:** F-017, F-019, F-020.

**What the generator does today:** emits flag schemas inconsistently across commands of the same generated CLI. Observed in SF360:
- Root command: no `--org` flag (F-017).
- `trust register`: requires `--org` (F-020).
- `sync`: no `--verbose` flag (F-019), despite `live-verify.sh` and README both documenting it.
- README/SKILL.md: show `--org prod` on virtually every command example, which then fail at runtime.

**What the generator should do:**
1. **Target-context flags must be consistent.** Every API has a "target" concept: Salesforce org, Linear workspace, HubSpot portal, Cal.com account. The generator should pick ONE approach per CLI:
   - **Option A (recommended):** a persistent global `--<target>` flag on root, honored by every subcommand that needs it. Fallback to env var (`<SLUG>_<TARGET>` e.g. `SF_TARGET_ORG`). Fallback to a platform-native default source (`sf config get target-org`).
   - **Option B:** no global flag; each subcommand declares `--<target>` independently. Documentation must show the per-command form.
   - **Never mix both approaches.**
2. **Observability flags must be global and uniform.** `--verbose`, `--debug`, `--quiet` should either be persistent global flags or absent from all subcommands. The generator should not emit `--verbose` references in any emitted script unless it's declared on the root command.
3. **Doc-flag matrix consistency check.** Before shipping, the generator diffs every `--flag` referenced in `README.md`, `SKILL.md`, `scripts/`, and `docs/` against the actual cobra command tree. Any mismatch is a blocker.

**How to validate:**
- Lint step: `printing-press validate-flags <generated-dir>` parses Go source for declared flags, parses markdown/scripts for referenced flags, emits a diff. Zero diff = pass.
- Smoke test: every fenced code block in README.md and SKILL.md that starts with `<cli-binary-name>` runs against the CLI's `--mock` mode and exits with non-usage-error.

**Why it's systemic:** this is the class of bug most likely to make Benioff (or any discerning user) dismiss the tool. The README promises behavior the binary doesn't have.

---

### U4 — Plan document auth matrix

**Rationale:** F-001.

**What the generator does today:** plan documents (generated per CLI under `docs/plans/`) include a single `auth login` command path that assumes production credentials.

**What the generator should do:**
Emit an **Authentication Matrix** section in every plan document, covering the target platform's environment separation model. Per-platform:

| Platform | Environments to list |
|---|---|
| Salesforce | Production, Sandbox (generic + My Domain URL form), Developer Edition |
| HubSpot | Private app token, OAuth flow, EU datacenter override |
| Cal.com | API key, self-hosted, team vs personal |
| Any SaaS | At minimum: prod credentials + test/staging credentials (if API supports it) |

The generator maintains a per-platform auth matrix template; plan docs interpolate.

**How to validate:**
Before shipping, confirm the plan document contains at least 2 auth rows (prod + one other) for any platform whose API differentiates environments.

**Why it's systemic:** Matt's plan document instructed Trent to test against sandbox but gave him the prod-only login command. This will happen to every plan the generator produces for a platform with environment separation, which is most of them.

---

### U5 — Plan document infrastructure prerequisite step

**Rationale:** F-003.

**What the generator does today:** plan documents leap straight to "run the first CLI command" with no consideration for infrastructure that must be deployed into the target platform first.

**What the generator should do:**
Emit a **Step 0.5: Platform Prerequisites** section in every plan document that lists:
- Any custom metadata / schema objects the CLI depends on (e.g., Salesforce custom objects)
- Any permissions / roles the CLI requires (e.g., permission sets, API scopes)
- Any one-time platform configuration (e.g., enable a feature toggle, install a connected app)
- The exact commands to perform each

For CLIs that don't need setup (HubSpot with a private app token needs none), the section can be explicitly empty with text "No prerequisites — proceed to Step 1."

**How to validate:**
- Every CLI that ships metadata/ or apex/ or schema/ directory MUST have a non-empty Step 0.5 in its plan document.
- Generator's metadata emitter must register "this metadata exists, so the plan needs a Step 0.5 deploy reference" with the plan emitter.

**Why it's systemic:** Matt's plan said "run `scripts/live-verify.sh`" but the script's Check 11 reads `SF360_Bundle_Audit__c` which doesn't exist until metadata is deployed. This cross-emitter dependency ("metadata emitter + plan emitter must coordinate") would affect any generator output that ships its own infra.

---

### U6 — Plan document seed-helper boilerplate

**Rationale:** F-015.

**What the generator does today:** plan documents assume the target platform already has "rich test data." For Developer sandboxes, brand-new test tenants, or fresh accounts, this is false.

**What the generator should do:**
For any CLI whose verification plan includes record-creating write verbs (update/create/upsert/delete), emit a **seed helper scaffold** alongside the plan:

1. `scripts/seed-discover.sh` — queries the platform's own describe/schema API to identify required-field constraints per sobject/resource type.
2. `scripts/seed-minimal.<ext>` — creates minimum-viable test fixtures. For Salesforce: Apex anonymous script. For HubSpot: Node/curl POST script. For Cal.com: bookings API calls.
3. `scripts/seed-run.sh` — orchestrates discover → user-editable seed → execute → print ID exports.

The generator maintains a per-platform seed-helper skeleton; each generated CLI gets a customized version.

**How to validate:**
- Any CLI whose `spec.yaml` / `.printing-press.json` declares write capabilities must ship `scripts/seed-run.sh`.
- Seed-run.sh must produce valid ID exports that the verify script can consume.

**Why it's systemic:** the "generic seed" Trent wrote during live-verify required 5 rounds of debugging against NFC's custom validation rules (F-015). A discover-first pattern would have caught all five on the first pass. Any CLI targeting a platform with record-level customizations (virtually all CRMs, ticketing systems, PM tools) faces the same class of friction.

---

### U7 — Bash script templates: `set -u`-safe array expansions

**Rationale:** F-018.

**What the generator does today:** emits bash scripts (live-verify.sh, seed helpers, setup scripts) that use `set -euo pipefail` **and** unchecked `${array[@]}` expansions. When the array is unset (typical on early exit before any tasks are tracked), bash crashes during the cleanup trap, masking the underlying failure.

**What the generator should do:**
- Lint emitted bash scripts before writing them. Specifically, reject any `${name[@]}` that isn't guarded by `${name[@]:-}` or wrapped in `[[ ${#name[@]} -gt 0 ]]`.
- Maintain a canonical bash template for cleanup-trap-style scripts. Every `trap <fn> EXIT` paired with a for-loop over an accumulator array must use the safe expansion form.
- Add these patterns to a generator-level "emitted-bash linter" checklist:
  - `set -u` + unchecked array expansion → fail
  - `trap ... EXIT` + mutation commands → must capture original state before first mutation
  - `for x in $(sf data query ...)` → must use `while read` with IFS guards instead (word-splitting bug)
  - `kill`/`rm -rf` commands inside the trap → must be prefixed with existence checks

**How to validate:**
- `shellcheck` run as part of generator pre-ship step; treat SC2206, SC2128, SC2068 as errors, not warnings.
- Unit test: spawn a generated live-verify.sh, send SIGINT mid-run, verify cleanup fires without crashing.

**Why it's systemic:** every Printing Press CLI likely ships some form of live-verification or integration-test bash script. The cleanup-trap-safety pattern applies uniformly.

---

### U8 — Pre-ship doc command smoke-test

**Rationale:** F-017 (doc aspect), F-019 (doc aspect).

**What the generator does today:** ships README.md, SKILL.md, and plan documents with command examples that fail at runtime (unknown flag, unknown command, wrong binary name, etc.).

**What the generator should do:**
Before declaring a generated CLI "ready to publish" in `registry.json`:

1. Extract every fenced code block whose first token matches the CLI's binary name.
2. For each line:
   - Strip pipeable trailing args (`| jq …`, `> /tmp/…`).
   - Substitute placeholder IDs (`001xx000003DGb2AAG`, `<alias>`, etc.) with values acceptable to `--mock`.
   - Execute the command with `--mock` (so no credentials required).
3. Assert exit code is NOT `2` (usage error) AND NOT `127` (command not found).
4. Any failure is a generator-level blocker — publish aborts with the failing command surfaced to the operator.

Variant for MCP parity: the same doc-smoke-test suite runs each SKILL.md command through the MCP server (`salesforce-headless-360-pp-mcp`) via a test harness.

**How to validate:**
- The generator ships a `printing-press verify-docs <generated-dir>` command that runs U8 locally.
- CI for `mvanhorn/printing-press-library` runs `verify-docs` on every CLI in the `library/` catalog on every PR.

**Why it's systemic:** the SF360 README has ≥10 code blocks that fail at runtime. Any generated CLI could ship with silent doc-code drift without this check. This is the upgrade with the highest leverage — it would have caught F-017, F-019, and several others at the moment of generation rather than months later during live verification.

---

## Prioritization

If the Printing Press maintainer has limited bandwidth, the recommended order:

1. **U8 (doc smoke-test)** — highest leverage, catches everything else automatically.
2. **U3 (flag schema consistency)** — closely related to U8; together they prevent the most visible class of CLI failure.
3. **U1 (layout emitter)** — fixes the "metadata deploy doesn't work from a fresh clone" class.
4. **U2 (schema compliance)** — fixes the "deploy partial-fails with cryptic errors" class.
5. **U5 (plan infra prereq)** — one-line template change per platform, high impact.
6. **U7 (bash script linter)** — cheap to add, improves all test scripts.
7. **U4 (auth matrix)** — template expansion.
8. **U6 (seed helpers)** — biggest scope (new scripts per platform), but unlocks realistic verification for any customized CRM-style platform.

## Non-upgrades (filtered out)

Listed here so the maintainer can confirm they were considered and correctly excluded:

- **F-002** (Salesforce sandbox password snapshot) — platform-specific.
- **F-004** (`FLS.AllowFieldWrite` param unused) — Go code bug in this CLI, not a generator pattern.
- **F-005** ("Log In" shortcut SSO) — Salesforce UI-specific.
- **F-006** (NFC org Sandbox Access group) — specific org configuration.
- **F-008** (Task/Event Activity-object metadata) — Salesforce-specific metadata limitation. However, the generator *could* maintain a per-platform "forbidden metadata combinations" list — arguably part of U2 extended scope.
- **F-011** (User lookup cascade) — Salesforce-specific.
- **F-016** (W3 Task idempotency cascade) — derived from F-008.

## Acceptance Criteria (for the Printing Press maintainer, not this repo)

- [ ] U1–U8 each have a linked issue or PR in `mvanhorn/cli-printing-press`
- [ ] Each upgrade's validation step runs in CI on the generator repo
- [ ] Re-running generation on `salesforce-headless-360-pp-cli`'s spec with the upgraded generator produces output that `scripts/live-verify.sh` can consume without manual intervention

## Sources & References

- **Origin:** [`/tmp/sf360-findings/README.md`](/tmp/sf360-findings/README.md) (20 findings, live verification against NFC TrentDev1 Developer sandbox, 2026-04-22 through 2026-04-24, tester Trent Matthias)
- **Companion:** [`2026-04-24-001-fix-sf360-live-verify-findings-plan.md`](2026-04-24-001-fix-sf360-live-verify-findings-plan.md) — local fixes for this CLI
- **Generator repo:** https://github.com/mvanhorn/cli-printing-press
- **This repo:** https://github.com/mvanhorn/printing-press-library
- **Driving PR:** https://github.com/mvanhorn/printing-press-library/pull/111
