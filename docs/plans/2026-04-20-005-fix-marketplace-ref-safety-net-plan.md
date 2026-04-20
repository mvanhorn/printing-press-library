---
title: "fix: Add ref:main to marketplace.json as git-subdir install safety net"
type: fix
status: active
date: 2026-04-20
---

# fix: Add ref:main to marketplace.json as git-subdir install safety net

**Target repo:** printing-press-library

## Overview

The plugin's `.claude-plugin/marketplace.json` declares the plugin source as `git-subdir` with `path: plugin` but no `ref` or `sha`. During a live upgrade session (2026-04-20), the cache directory that Claude Code expects at `~/.claude/plugins/cache/printing-press-library/printing-press-library/<version>/` was never populated after `/plugin` enable and four subsequent version bumps (1.1.14 -> 1.1.21 -> 1.1.22 -> 1.1.23). `installed_plugins.json` recorded the version but the extraction step silently skipped. Result: `/pp` autocomplete returned nothing because the plugin files were never where Claude Code looked for them.

Root cause is not definitively known. Two contributing theories:

1. Rapid version churn confused the installer (fresh installs would go 0 -> 1.1.23 in one step; upgrade churn is a less-tested path).
2. Our marketplace manifest lacks `"ref": "main"`. Surveying the 22 git-subdir plugins in `claude-plugins-official`, 20 include `"ref": "main"` (or `"master"`) and many also pin `"sha"`. Only two (semgrep and ours) omit both. Semgrep's install path isn't exercised locally to confirm whether it works.

Adding `"ref": "main"` is a cheap defensive fix that aligns with the majority pattern, without committing to a larger repo restructure. This plan ships only that change.

## Problem Frame

One user (the maintainer) hit the missing-cache failure on upgrade. Fresh-install behavior for other users is unverified. We have two options on the table:

- Option 1 (this plan): add `"ref": "main"` to marketplace.json. One-line change, one PR, ships immediately.
- Option 3 (deferred): flatten the repo so the manifest lives at the root and `git-subdir` is no longer needed. Eliminates the whole class of subdir-extraction failure modes but requires moving all 23 command files + 22 skill dirs up one level, updating `tools/generate-skills` paths, updating `AGENTS.md`, and re-verifying the generator.

This plan picks Option 1 as the minimum-viable safety net. Option 3 remains tracked as a follow-up if this doesn't resolve the issue for other users.

## Requirements Trace

- R1. `.claude-plugin/marketplace.json` MUST declare `"ref": "main"` in the plugin's source block, matching the shape used by the majority of git-subdir plugins in the official marketplace.
- R2. Existing marketplace fields (`source: "git-subdir"`, `url`, `path: "plugin"`) MUST be preserved verbatim; only `ref` is added.
- R3. The change MUST NOT require a `plugin/.claude-plugin/plugin.json` version bump; `marketplace.json` is not version-gated the same way.
- R4. A brief note MUST be added to `AGENTS.md` explaining the `ref` requirement so future marketplace edits don't drop it.
- R5. The plan MUST explicitly defer the repo-flattening option (3) to a follow-up plan rather than attempting it here.

## Scope Boundaries

- Not restructuring the repo to move the manifest out of `plugin/`.
- Not adding a `sha` pin (would require per-release updates; the `ref: main` pattern without `sha` is sufficient and matches plugins like `amazon-location-service` and `aws-amplify`).
- Not changing `plugin/.claude-plugin/plugin.json` contents.
- Not changing the generator, skills, commands, or library CLIs.
- Not filing an upstream Claude Code bug report as part of this plan (separately tracked).

### Deferred to Separate Tasks

- Repo flatten (Option 3): if the `ref` fix doesn't resolve the issue for new users, follow up with a plan to move `plugin/` contents to the repo root and drop `git-subdir` from marketplace.json.
- Claude Code bug report: file separately after observing whether the `ref` fix alone resolves the symptom.

## Context & Research

### Relevant Code and Patterns

- `.claude-plugin/marketplace.json`: the file to edit.
- Reference shape (majority pattern): `/Users/mvanhorn/.claude/plugins/marketplaces/claude-plugins-official/.claude-plugin/marketplace.json` has 22 git-subdir plugins; 20 of them use `"ref": "main"` or `"ref": "master"`. Examples worth mirroring are `amazon-location-service`, `aws-amplify`, `stripe`, and `pydantic-ai`, all of which use `ref` without `sha`.
- `AGENTS.md` "Repository layout" and "Keeping plugin/skills in sync" sections are the natural places to document the `ref` requirement.

### Institutional Learnings

- 2026-04-20 live session captured the missing-cache symptom and the diagnostic evidence (installed_plugins.json pointed at a non-existent cache directory; marketplace clone had all files; rapid version churn preceded the failure).
- Prior PR #100 landed the commands directory; PR #101 added `commands: ./commands/` to plugin.json. Neither PR touched marketplace.json. The `ref` gap predates this session.

### External References

- Claude Code plugin marketplace schema: https://anthropic.com/claude-code/marketplace.schema.json (referenced by last30days-skill's marketplace.json `$schema` key).

## Key Technical Decisions

- Decision: Add only `"ref": "main"`, not `"sha"`. Rationale: pinning `sha` would require updating marketplace.json on every release, and the `ref: main` pattern is sufficient per the majority of official plugins.
- Decision: Do not bump `plugin/.claude-plugin/plugin.json` version. Rationale: the plugin version hasn't changed; only the marketplace descriptor has. The bump is only warranted when plugin content changes.
- Decision: Document the `ref` requirement in AGENTS.md, not just fix silently. Rationale: future marketplace edits that drop `ref` would re-introduce the failure mode; the docs note makes that explicit.
- Decision: Keep this plan Lightweight. Rationale: one-file change with a clear diff, no behavioral logic, no test surface. A larger plan would be theater.

## Open Questions

### Resolved During Planning

- Q: Should we also pin `sha`? A: No. Official plugins that update frequently (e.g. aws-amplify) skip `sha` and rely on `ref`. Pinning creates release burden for no additional safety.
- Q: Should we bump the plugin version? A: No. Plugin content is unchanged.
- Q: Should we add a `$schema` key like last30days? A: Optional nice-to-have but out of scope. Follow-up if desired.

### Deferred to Implementation

- Q: Whether to test the fix on a fresh machine before merging. Answer: not feasible within this plan; we'll land the change and watch for user reports. If reports persist, Option 3 activates.

## Implementation Units

- [ ] **Unit 1: Add ref:main to marketplace.json + note in AGENTS.md**

**Goal:** Align our marketplace.json source block with the majority git-subdir shape, and document why `ref` is required so future edits don't drop it.

**Requirements:** R1, R2, R3, R4

**Dependencies:** None

**Files:**
- Modify: `.claude-plugin/marketplace.json`
- Modify: `AGENTS.md`

**Approach:**
- In `.claude-plugin/marketplace.json`, add `"ref": "main"` to the source block for the single plugin entry. Preserve field order as seen in the official marketplace: `source`, `url`, `path`, then `ref`.
- In `AGENTS.md`, add a short note (one or two sentences) in the "Repository layout" or a new "Marketplace manifest" subsection explaining that git-subdir plugins require `"ref": "main"` and pointing at the 2026-04-20 incident as the reason.
- No plugin.json change. No generator change.

**Patterns to follow:**
- `amazon-location-service` and `aws-amplify` source blocks in `/Users/mvanhorn/.claude/plugins/marketplaces/claude-plugins-official/.claude-plugin/marketplace.json` for field ordering.

**Test scenarios:**
- Test expectation: none -- pure marketplace-metadata change; no code paths changed. The fix's effectiveness can only be verified by observing a fresh install on a machine that doesn't have the plugin cached, which is outside the scope of an automated test.

**Verification:**
- Git diff for `.claude-plugin/marketplace.json` shows only the addition of `"ref": "main"` inside the source object.
- `AGENTS.md` diff adds a brief note pointing at the ref requirement.
- `python3 -m json.tool .claude-plugin/marketplace.json` parses cleanly (catches accidental trailing-comma breakage).

## System-Wide Impact

- **Interaction graph:** No application code touched. Only Claude Code's plugin-install flow reads this file.
- **Error propagation:** N/A.
- **State lifecycle risks:** Existing installs (the maintainer's) already have the version recorded in `installed_plugins.json`; a manual disable+re-enable may still be required for the one user who hit the original failure. New users are the beneficiaries.
- **API surface parity:** None affected.
- **Integration coverage:** CI doesn't exercise this file today; the existing `verify-skills.yml` workflow ignores it.
- **Unchanged invariants:** Plugin version, skills, commands, generator, library CLIs, and all downstream behavior are unchanged.

## Risks & Dependencies

| Risk | Mitigation |
|------|------------|
| The `ref` addition isn't actually the cause of the cache-missing failure, and new users still hit it. | The change is cheap regardless; if the symptom persists, the tracked follow-up (Option 3 repo flatten) is the next step. |
| Claude Code refreshes the marketplace but still doesn't extract files for existing installs. | Not solvable in this repo; the maintainer manually populated the cache for their machine. Document the symptom + workaround in AGENTS.md for anyone else who hits it. |
| Future marketplace.json edit drops the `ref` key. | Unit 1 adds the explicit AGENTS.md note to prevent this; PR review is the safety net. |

## Documentation / Operational Notes

- AGENTS.md gets a one-paragraph note about `ref: main` being required for git-subdir installs, citing the 2026-04-20 incident.
- No CHANGELOG today; if added later, note the marketplace safety-net at the next plugin version bump.
- No rollout staging needed.

## Sources & References

- 2026-04-20 conversation capturing the missing-cache symptom (root cause analysis).
- Claude plugins marketplace clones: `/Users/mvanhorn/.claude/plugins/marketplaces/` for comparison shapes.
- Related PRs: #100 (commands infra), #101 (commands path declaration).
- Repo convention: `AGENTS.md` sections "Repository layout" and "Keeping plugin/skills in sync".
