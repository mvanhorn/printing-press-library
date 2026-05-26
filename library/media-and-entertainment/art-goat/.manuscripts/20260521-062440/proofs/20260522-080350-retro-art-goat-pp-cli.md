# Printing Press Retro: art-goat (publish session)

## Session Stats
- API: art-goat
- Spec source: catalog (printed 2026-05-21, run 20260521-062440)
- Scorecard: 86/100 (from prior polish; this session did not re-run scoring)
- Verify pass rate: 100% (held through publish validate, 11/11 checks)
- Fix loops: 0 generation, 3 publish (cli-skills CI rejection; commit-author email; force-with-lease tracking-ref dance)
- Manual code edits: 1 adapter add (Harvard), 2 adapter removes (Rijks, Te Papa), 4 test-fixture sweeps, 3 help-text example swaps
- Features built from scratch: 0 (Harvard adapter copied the Rijks template; not a novel feature)

> This retro is a **publish-session retro**, not a print-session retro. The print of art-goat happened 2026-05-21 in a prior session and has two prior retros at `proofs/20260521-095145-…` and `proofs/20260521-150000-…`. The findings below are from operating the publish flow on 2026-05-22, not from the original print.

## Findings

### 1. Publish skill still regenerates and commits the cli-skills mirror — closure #1474 didn't stick (skill instruction gap)

- **What happened:** `/printing-press-publish` Step 6 has `if [ -f "$PUBLISH_REPO_DIR/tools/generate-skills/main.go" ]; then (cd "$PUBLISH_REPO_DIR" && go run ./tools/generate-skills/main.go); fi` at lines 442-443 of the locally installed `~/.claude/skills/printing-press-publish/SKILL.md`. Following the instruction regenerated the cli-skills/pp-art-goat/ mirror and committed it. The library's `Verify library conventions` check then rejected the PR with "This PR modifies generated artifacts that are regenerated post-merge." Removing the mirror from the commit and force-pushing made the check pass. Same failure pattern as `#1474` (closed 2026-05-16) and three earlier closures (`#668`, `#682`, `#664`).
- **Scorer correct?** N/A — instruction conflict, not a scorer finding.
- **Root cause:** The locally installed `printing-press-publish` SKILL.md has not absorbed the fix that closed `#1474`. Either the closure was speculative (no skill edit landed), or the skill was reverted, or the fix landed in a different distribution channel than the `~/.claude/skills/printing-press-publish/SKILL.md` users actually load.
- **Cross-API check:** Affects every fork PR to `mvanhorn/printing-press-library` for a brand-new CLI. The youtube retro (2026-05-15) documented the exact same failure on PR #585. art-goat publish (this session) hit it on PR #753 — three pushes wasted (force-push initial, force-push minus cli-skills, force-push with corrected author).
- **Frequency:** every fork PR for a brand-new CLI; recurs every time anyone outside `mvanhorn` ships a new CLI.
- **Fallback if the Printing Press doesn't fix it:** every external publisher hits the same N-push fix cycle. With prior retros surfacing this 4 times and closure #1474 not propagating to local skills, the fallback is provably unreliable.
- **Worth a Printing Press fix?** Yes — but the meta-fix is needed. The case has been made 4 times; the closure was insufficient. Reframing: ship a release of the publish skill that visibly verifies the regen-and-commit lines are gone (lines 442-443), and ensure user-distribution channels pick up the edit. Reopen #1474 with verification evidence.
- **Inherent or fixable:** Fixable. Drop the regen-and-commit from Step 6 of `~/.claude/skills/printing-press-publish/SKILL.md` and verify the locally distributed copy reflects the change.
- **Durable fix:** Re-confirm the skill edit landed and reaches the user-distribution path. The closing-comment + bash audit on the installed skill should be part of `#1474`'s re-verification.
- **Test:** positive — `grep -c "generate-skills/main.go" ~/.claude/skills/printing-press-publish/SKILL.md` returns 0 on a freshly installed skill. negative — a fork-PR-for-new-CLI publish reaches `Verify library conventions` green without `cli-skills/` in the diff.
- **Evidence:** PR #753 force-push 1 → `Verify library conventions: FAILURE` at run `26294881597` ("##[error]This PR modifies generated artifacts... cli-skills/pp-art-goat/SKILL.md"). Removed cli-skills/ from commit, force-pushed → next run pending then pass. Source: `~/.claude/skills/printing-press-publish/SKILL.md:442-443` still has the regen-and-commit `if` block.
- **Related prior retros:**
  - `youtube` retro (2026-05-15, finding F1) — `aligned`. Same root cause, same proposed fix, then closed as `#1474`.
  - Closed issues `#1474`, `#668`, `#682`, `#664` — all `aligned`, all in the same family.
  - This session's finding is the 4th raising of this finding family. Per the retro skill's Step D recurrence-cost check, this should NOT be filed fresh — instead, the new evidence is added as a comment on `#1474` requesting re-open + verification that the skill edit shipped to user-distribution.

### 2. Publish skill validates manifest `.printer` but doesn't set or validate the actual git commit author email (skill instruction gap)

- **What happened:** Step 6 has defense-in-depth: it reads `.printer` and `.printer_name` from `.printing-press.json` and fails if they're empty or sentinel. But Step 8 (commit) does not set or validate `git config user.email` against the GitHub noreply form. On this session's publish, I committed with `user.email = "<user-email-redacted>"` (a real personal email) because that's what the shell session's user-global git config had. The user caught it on review and asked me to use `<user-noreply-redacted>` instead. Because the target repo is public, the real email lands in a public commit's metadata without the skill flagging it.
- **Scorer correct?** N/A — the secret/PII scan runs over file contents, not git commit metadata. Commit author email isn't checked by any current scorer.
- **Root cause:** Step 6 already encodes the principle "validate identity before public-PR" (the `PRINTER == "USER"` sentinel check, the empty-string guards). Step 8 implicitly trusts `git config user.email` to be a noreply form, which is not a safe default. The skill could mirror the Step 6 pattern: derive the noreply form from `gh api user --jq .id` + `gh user.login`, set `git config user.email` in the managed clone, and refuse to commit if a real-looking email is configured anywhere upstream.
- **Cross-API check:** Affects every publish from a user whose global `git config user.email` is a real email — which is the default for most developers. Independent of which CLI is being published; the failure surface is the publish skill itself, not the CLI being printed.
- **Frequency:** every publish; the PII leak shape recurs across CLIs whenever the user's git config isn't already set to the noreply form.
- **Fallback if the Printing Press doesn't fix it:** the user must manually set `git config user.email` per publish-managed-clone, or rely on global override. Reliability is low because the publish skill does the commit — the user has no natural point to intervene unless they read every commit author line on every PR.
- **Worth a Printing Press fix?** Yes. The skill is already in the identity-validation business (Step 6 printer guards); adding commit-author validation extends an existing pattern by 4 lines.
- **Inherent or fixable:** Fixable. Set `git config user.email` to the noreply form in the managed clone after the first-time setup; validate the configured value before each commit.
- **Durable fix:** In Step 5 (Managed Clone — first-time setup), after `git clone`, set:
  ```bash
  GH_ID=$(gh api user --jq '.id')
  GH_LOGIN=$(gh api user --jq '.login')
  git -C "$PUBLISH_REPO_DIR" config user.name "$GH_LOGIN"
  git -C "$PUBLISH_REPO_DIR" config user.email "${GH_ID}+${GH_LOGIN}@users.noreply.github.com"
  ```
  In Step 8 (Branch, Commit, and PR), before the `git commit`, assert that the configured `user.email` matches `^[0-9]+\+[A-Za-z0-9_-]+@users\.noreply\.github\.com$`. Refuse to commit with a one-line fix hint if it doesn't.
- **Test:** positive — after first-time setup, `git -C "$PUBLISH_REPO_DIR" config user.email` ends in `@users.noreply.github.com`. negative — a publish with `git config --global user.email="<placeholder-email>"` does NOT produce a commit author of `<placeholder-email>`; the skill refuses or auto-overrides.
- **Evidence:** PR #753 commit `a8aaa3b6` had author `Justin Fu <<user-email-redacted>>` because the shell session's git config was global+real. User flagged on review (2026-05-22). Fixed by setting `git config user.name "justinwfu"; git config user.email "<user-noreply-redacted>"` locally + recommitting + force-pushing to `b6a740b3`.
- **Related prior retros:** None match. Adjacent open issue: `#1124` (`generator: auto-derive printer from gh whoami when git config github.user is unset`) — `extends`. Same identity-derivation principle but #1124 targets the generator's manifest `.printer` field, while this finding targets the publish skill's git commit author. Different surface, same idea (derive identity from `gh whoami`, don't trust local git config).

## Prioritized Improvements

### P2 — Medium priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| F2 | Publish skill should set + validate commit-author noreply email | skill | every publish | None — user has no natural intervention point | small | none needed |

### Skip
| Finding | Title | Why it didn't make it (Step B / Step D / Step G) |
|---------|-------|--------------------------------------------------|
| F1 | Publish skill cli-skills regen contradiction | Step D — raised 4 times across retros; closure #1474 didn't stick. Comment on existing closed issue with new evidence requesting re-open + skill-distribution verification, rather than filing a 5th duplicate. |

### Dropped at triage
| Candidate | One-liner | Drop reason |
|-----------|-----------|-------------|
| Te Papa adapter runaway pagination loop | Post-`q=hasRepresentation:*` mapping path drops 100% of records, sending sync into infinite pagination | `printed-CLI` — adapter bug in one CLI; not generalizable to a generator template |
| Rijks signup URL stale (`data.rijksmuseum.nl/object-metadata/api/` 404s) | Adapter's auth-error string points at a dead signup URL | `unproven-one-off` — one adapter; can't name 3+ other CLIs with stale signup URLs in error strings |
| Harvard adapter manual-add was 2-4 hours of work | A new source adapter is hand-coded against the Rijks template | `printed-CLI` / normal iteration — building a new source-of-a-federation is exactly what the press's "non-goal is flawless" stance describes |
| Templated `sync` is a no-op for federated CLIs | Generator emits a `sync` subcommand that delegates to a per-resource API the federation pattern doesn't use | `unproven-one-off` — can't name 3+ federation-pattern CLIs in the catalog; coffee-goat and art-goat are the only two and both share lineage |
| Test fixtures still reference removed source slug | After deleting an adapter dir, test files keep `Source: "rijks"` strings until manually swept | `printed-CLI` — pure per-CLI hygiene after a hand-edit; no general pattern |
| pii-polish artifact leaks absolute `cli_dir` | `.printing-press-pii-polish.json` writes the absolute filesystem path while `.printing-press-tools-polish.json` uses a `<cli-dir>` placeholder | already filed pre-retro as `mvanhorn/cli-printing-press#1840`; aligned with open `#1587` (manifest spec_path leak) |

## Work Units

### WU-1: Set + validate commit-author noreply email in publish skill (from F2)
- **Priority:** P2
- **Component:** skill
- **Goal:** Publish skill auto-derives the GitHub noreply email from `gh api user` and sets `git config user.email` in the managed clone, then defensively validates the configured value matches the noreply pattern before each `git commit`. Closes the identity-leak gap that the Step 6 `.printer` validation already addresses for the manifest but not for the commit author.
- **Target:** `~/.claude/skills/printing-press-publish/SKILL.md` — Step 5 (Managed Clone, first-time setup section after `git clone`) and Step 8 (Branch, Commit, and PR, before `git commit`).
- **Acceptance criteria:**
  - positive test: after first-time setup, `git -C "$PUBLISH_REPO_DIR" config user.email` ends in `@users.noreply.github.com` and `git -C "$PUBLISH_REPO_DIR" config user.name` matches `gh api user --jq .login`.
  - positive test: a publish run with `git config --global user.email="<placeholder-email>"` still produces a commit with author email matching `^[0-9]+\+[A-Za-z0-9_-]+@users\.noreply\.github\.com$`.
  - negative test: if `gh api user --jq .id` fails (no auth), the skill stops with a clear error rather than committing under a real email.
- **Scope boundary:** Does NOT touch the manifest's `.printer` derivation (that's `#1124`'s territory in the generator). Only touches the publish skill's git config + commit-author validation.
- **Dependencies:** None.
- **Complexity:** small.

## Anti-patterns
- **None new this session.** The "follow the skill verbatim and hit a known closed issue" anti-pattern is a meta-finding: the existence of closure `#1474` and the recurrence of its symptom suggests retro closure verification could be tighter, but that's a process finding outside this retro's scope.

## What the Printing Press Got Right
- **Validation suite caught nothing on a 9-file delta.** All 11 validate checks (manifest, transcendence, phase5, go mod tidy, govulncheck, go vet, go build, --help, --version, verify-skill, manuscripts) passed cleanly on a CLI with a new adapter + two removed adapters + four edited test files + three edited help-strings. The validate surface is doing real work and not flagging false positives.
- **Polish's `sync → sources sync` redirect was on-brand.** Polish noticed the templated `sync` was a no-op for the federation case and redirected docs to `sources sync` without trying to delete the generator-owned file. Recognizing that fix and flagging it as a retro candidate (rather than editing the `DO NOT EDIT` file) was the right move.
- **`printing-press publish package` PII scan caught zero false positives on a tree with absolute user paths.** The `.printing-press-pii-polish.json` leak that I scanned for manually was missed by the press scan because the file isn't on the press scan's allowlist — but the press scan also didn't flag anything else, including the legitimate use of the user's GitHub handle in copyright headers and the absolute paths in some manuscript artifacts. Good precision in the current implementation.
