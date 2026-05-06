---
title: "feat: hand off @mvanhorn/printing-press npm setup to Trevin via temp token"
type: feat
status: active
date: 2026-05-05
origin: https://www.proofeditor.ai/d/c8v9n24h
---

# feat: hand off @mvanhorn/printing-press npm setup to Trevin via temp token

## Summary

Get `@mvanhorn/printing-press` published to npm with the smallest possible amount of Matt-time, by handing a short-lived granular publish token to Trevin so he can drive the initial publish, the npm Trusted Publishing migration, and the workflow PR himself. Matt's involvement is reduced to two unavoidable touchpoints (issue token, add maintainer) totaling ~3 minutes; everything else is Trevin-owned.

---

## Problem Frame

Trevin's Proof doc lays out a clean path: publish v0.1.0 manually once, configure npm Trusted Publishing (OIDC), and let GitHub Actions handle every future release on `npm-v*` tags. The doc assumes Matt drives the whole sequence locally. That assumption is the thing we're rejecting — Matt wants Trevin able to develop with his agent end-to-end with near-zero hand-offs.

Two npm-side facts force the minimum Matt-touch floor:

1. The first publish must authenticate as the scope owner. A granular token issued by Matt is the cheapest delegation primitive.
2. `npm owner add` and Trusted Publishing UI configuration both require a maintainer of the package. Until tmchow is added as a maintainer, Trevin cannot configure OIDC himself. That requires the package to exist *first*, so Matt's `npm owner add` step must happen *after* Trevin's first publish — chicken and egg, can't be eliminated.

Net: Matt cannot literally do zero. He can do 3 minutes split across two short windows.

---

## Requirements

- R1. `@mvanhorn/printing-press` v0.1.0 lives on the public npm registry and `npx -y @mvanhorn/printing-press install` works for end users.
- R2. Future releases trigger automatically when any maintainer pushes an `npm-v*` tag — no manual `npm publish` and no long-lived `NPM_TOKEN` in repo secrets.
- R3. `tmchow` is a package maintainer with publish rights and can configure Trusted Publishing.
- R4. The temp token Matt issues has a TTL (90 days) so it self-cleans if forgotten. Revocation is nice-to-have, not required for the handoff to be considered done.
- R5. `.github/workflows/npm-publish.yml` uses GitHub OIDC (`id-token: write`) and contains no `NPM_TOKEN` reference after the migration lands.

---

## Scope Boundaries

- Not migrating any other repo (osc-status, paperclip, cli-printing-press) to npm distribution. Neither has an npm package today; this plan only sets up `printing-press-library`.
- Not redesigning the installer CLI itself. `npm/bin/pp.mjs` and `npm/src/` are already shipped; this plan publishes what exists.
- Not introducing semantic-release, changesets, or any auto-versioning tool. Releases stay manual-tag-driven per Trevin's plan.
- Not creating an `@mvanhorn` npm Organization. Personal scope is sufficient if Matt's npm username is `mvanhorn`. Org creation is only a fallback if it isn't (handled inline in U1).

### Deferred to Follow-Up Work

- Bump cadence and changelog automation: separate concern, Trevin can propose later if useful.
- Adding more maintainers beyond tmchow: not needed for this handoff.

---

## Context & Research

### Relevant Code and Patterns

- `npm/package.json` — already names the package `@mvanhorn/printing-press` v0.1.0 with `bin: pp` and proper `files:` allowlist.
- `npm/bin/pp.mjs` — the installer entry point users invoke via `npx`.
- `npm/src/cli.ts`, `npm/src/registry.ts` — installer logic; tests under `npm/tests/`.
- `.github/workflows/npm-publish.yml` — currently triggers on `npm-v*` tags, runs `npm ci && npm test && npm run build && npm publish --access public`, authenticates with `secrets.NPM_TOKEN`. This is the file that needs the OIDC swap.
- `AGENTS.md` (repo root) describes `npm/` as "the @mvanhorn/printing-press npm installer wrapper" and confirms this is the intended distribution surface.

### Institutional Learnings

- Matt's GitHub repo is private; tmchow is already a GitHub collaborator (verified). No GitHub-side access work needed.
- Repo secrets currently in place: `CLAW_TOKEN`, `GENERATOR_READ_TOKEN`, `RELEASE_WRITE_TOKEN`. No `NPM_TOKEN` exists yet, so removing it from the workflow won't break a current release — there is no current release.
- `npm whoami` returns nothing on Matt's machine — Matt is not currently logged into npm. `npm login` with 2FA is part of U1.

### External References

- npm Trusted Publishing (OIDC): https://docs.npmjs.com/trusted-publishers
- npm granular access tokens: https://docs.npmjs.com/about-access-tokens#about-granular-access-tokens — supports per-package scope, expiry, publish-only permission. This is the right primitive for the temp handoff.
- GitHub Actions OIDC for npm: workflow needs `permissions: id-token: write` and the `actions/setup-node` config that omits `NODE_AUTH_TOKEN`.

---

## Key Technical Decisions

- **Granular token over classic automation token.** Granular scopes to `@mvanhorn/printing-press` only — not a trust thing, just the right credential shape so the token can't accidentally be reused for something it wasn't meant for. Classic automation tokens grant publish across Matt's whole npm account, which is more reach than this hand-off needs.
- **Token TTL: 90 days, no urgency on revocation.** Matt trusts Trevin. The TTL exists so the token expires on its own if everyone forgets about it; nothing worse happens if it lingers until U7.
- **Trevin runs the initial publish from his local machine, not from a temporary GitHub Action.** Local publish is one command; setting up a temp Actions secret just to publish once would mean Matt has to write the token into GitHub secrets, which is more work for him, not less.
- **`npm owner add tmchow` happens after the first publish, not before.** npm rejects ownership commands on packages that don't exist. This sequencing is forced.
- **Trusted Publisher config is done by Trevin from the npm UI** once he's a maintainer. Matt could do it himself in 60 seconds, but the stated preference is to keep Matt-touchpoints minimal, and Trevin can own this step end-to-end.
- **Workflow PR removes `NPM_TOKEN` references entirely.** No fallback path. Once OIDC works, the long-lived token is dead; leaving it as a fallback creates a second auth surface to maintain.

---

## Open Questions

### Resolved During Planning

- *Do we need an npm Organization?* Only if Matt's npm username isn't `mvanhorn`. U1 contains a branch for that case (create org, transfer scope) but defaults to personal scope.
- *Can the temp token configure Trusted Publishing or add maintainers?* No. npm tokens authenticate publish/install only. UI-driven settings (trusted publishers, maintainer invites) require a logged-in maintainer session. This is why `npm owner add` stays Matt's job and why Trevin must be a maintainer before he configures OIDC.
- *Should we leave the existing `NPM_TOKEN` reference in the workflow as a fallback?* No — see Key Technical Decisions.

### Deferred to Implementation

- *Exact granular-token UI flow.* npmjs.com's settings UI shifts; whoever does U1 confirms the current path (Profile → Access Tokens → Generate New → Granular).
- *Whether the test publish in U6 needs a real version bump or `--dry-run` is enough.* Prefer a real `npm-v0.1.1` tag with a no-op patch bump (e.g., README touch-up) to verify end-to-end OIDC; `--dry-run` doesn't exercise the workflow.

---

## Implementation Units

### U1. Matt: log into npm, issue scoped temp token, share with Trevin

**Goal:** Produce a granular publish token Trevin can use for one publish, and confirm scope ownership.

**Requirements:** R1, R3, R4

**Dependencies:** None

**Files:** None (npm UI + 1Password/Signal/secure-share channel only)

**Approach:**
- Run `npm login` locally; complete 2FA. Confirm `npm whoami` returns `mvanhorn`. If it returns a different username, create a free npm Organization at npmjs.com/org/create with name `mvanhorn` (per Trevin's plan, Step 1) so the `@mvanhorn` scope resolves.
- In npmjs.com → Profile → Access Tokens → Generate New Token → Granular: scope to packages matching `@mvanhorn/printing-press`, permission Read and write (Publish), expiry 90 days. Copy the token.
- Send the token to Trevin via whatever channel is convenient (Slack DM, email, 1Password share — pick what gets it there fastest). Include a pointer to this plan so he has the runbook.

**Patterns to follow:**
- npm 2FA is required on Matt's account per Trevin's plan preconditions.

**Test scenarios:**
- Happy path: `npm whoami` returns `mvanhorn`; the new token appears in npm Access Tokens UI with the correct scope, permissions, and expiry; Trevin confirms receipt out-of-band.
- Edge case: Matt's npm username differs from `mvanhorn` → Org creation branch runs, scope is then owned by the org, token is generated against the org-owned package name.

**Verification:**
- Matt has confirmation from Trevin that the token works (Trevin can authenticate; doesn't have to publish yet).

---

### U2. Trevin: validate the package and run the initial publish

**Goal:** Ship `@mvanhorn/printing-press` v0.1.0 to public npm using Matt's temp token.

**Requirements:** R1

**Dependencies:** U1

**Files:**
- Modify (if needed): `npm/CHANGELOG.md` to mark v0.1.0 release date
- No other code changes — packaging is already correct

**Approach:**
- From a fresh clone: `cd printing-press-library/npm && npm ci && npm test && npm run build`. All green is required before publishing.
- Run `npm pack --dry-run` and review the file list against `npm/package.json`'s `files:` allowlist. Confirm `bin/`, `dist/src/`, `CHANGELOG.md`, `README.md` are present and `tests/`, `src/`, `tsconfig.json`, `node_modules/` are absent.
- Authenticate using the temp token as `NODE_AUTH_TOKEN` (or `npm config set //registry.npmjs.org/:_authToken` for the session) and run `npm publish --access public`.
- Verify the published artifact: `npm view @mvanhorn/printing-press` shows v0.1.0; `npx -y @mvanhorn/printing-press --help` (or whatever the CLI surfaces) executes from a clean machine.
- Ping Matt that initial publish is done and he can run U3.

**Patterns to follow:**
- Existing `npm test` script chains build + node test runner; no changes needed.

**Test scenarios:**
- Happy path: tests pass, `npm pack --dry-run` lists exactly the expected files, `npm publish` succeeds, package page exists at npmjs.com/package/@mvanhorn/printing-press, `npx -y @mvanhorn/printing-press install --help` runs without error.
- Edge case: `npm test` fails → STOP. Do not publish. File a bug, fix the build, retry. Publishing a broken v0.1.0 burns the version forever (npm forbids overwrite).
- Error path: `npm publish` returns 403 → token scope or permission is wrong; check token settings before retry. Do NOT escalate scope of the token.
- Error path: `npm publish` returns 402 (payment required) → npm thinks it's a private package; ensure `--access public` is on the command line.

**Verification:**
- `curl -sI https://registry.npmjs.org/@mvanhorn/printing-press/0.1.0` returns 200.
- `npx -y @mvanhorn/printing-press` runs without error from a directory with no node_modules.

---

### U3. Matt: add tmchow as npm maintainer

**Goal:** Promote tmchow from "no npm relationship" to "package maintainer" so he can configure Trusted Publishing in U4.

**Requirements:** R3

**Dependencies:** U2 (package must exist before `npm owner add` works)

**Files:** None (npm CLI command only)

**Approach:**
- Run `npm owner add tmchow @mvanhorn/printing-press`. Complete 2FA challenge if prompted.
- Confirm with `npm owner ls @mvanhorn/printing-press` that both `mvanhorn` and `tmchow` appear.
- Notify Trevin he can proceed to U4.

**Test scenarios:**
- Happy path: `npm owner ls` shows two entries.
- Edge case: tmchow's npm username differs from `tmchow` → confirm with Trevin out-of-band before running the command. Run with the verified username.
- Error path: `npm owner add` returns "user not found" → Trevin doesn't have an npm account; Trevin creates one, then Matt retries.

**Verification:**
- `npm owner ls @mvanhorn/printing-press` lists `tmchow`.

---

### U4. Trevin: configure Trusted Publishing on npmjs.com

**Goal:** Wire the npm package to GitHub Actions OIDC so the workflow can publish without a token.

**Requirements:** R2, R5

**Dependencies:** U3

**Files:** None (npm UI only)

**Approach:**
- Log into npmjs.com as tmchow.
- Navigate to npmjs.com/package/@mvanhorn/printing-press → Settings → Trusted Publishers → Add.
- Configure:
  - Provider: GitHub Actions
  - Organization or user: `mvanhorn`
  - Repository: `printing-press-library`
  - Workflow filename: `npm-publish.yml`
  - Environment: leave blank unless U5 introduces one.
- Save.
- Screenshot the configured trusted publisher (for the verification step in Trevin's Proof doc).

**Test scenarios:**
- Happy path: Trusted Publishers list shows the GitHub config; UI confirms it's active.
- Edge case: tmchow can't see the Settings tab → he isn't a maintainer; back up to U3 and verify ownership.

**Verification:**
- npm UI reflects an active trusted publisher tied to `mvanhorn/printing-press-library` + `npm-publish.yml`.

---

### U5. Trevin: PR the workflow over to OIDC

**Goal:** Update `.github/workflows/npm-publish.yml` so it publishes via OIDC instead of `NPM_TOKEN`.

**Requirements:** R2, R5

**Dependencies:** U4

**Files:**
- Modify: `.github/workflows/npm-publish.yml`

**Approach:**
- Add top-level (or job-level) permissions block with `id-token: write` and `contents: read`. Job-level scoping is preferred for least privilege.
- Remove the `env: NODE_AUTH_TOKEN: ${{ secrets.NPM_TOKEN }}` block from the `Publish` step. With OIDC, `setup-node` and `npm publish` negotiate the token automatically when Trusted Publishing is configured for the package.
- Confirm `actions/setup-node` is at a version that supports OIDC for npm (v4+ does; the workflow already uses `actions/setup-node@v6`).
- Open a PR with title `chore(ci): migrate npm publish workflow to Trusted Publishing (OIDC)` and a body that links this plan and Trevin's Proof doc.

**Patterns to follow:**
- Existing workflow already uses `working-directory: npm` for build/test/publish steps — preserve that.
- Existing workflow already pins to `actions/checkout@v6` and `actions/setup-node@v6` — leave the pins alone.

**Test scenarios:**
- Happy path: PR diff is small (one permissions block added, one env block removed); CI checks on the PR pass.
- Edge case: workflow lints/checks complain about missing `id-token` permission scope → confirm permissions block is at the right level (job, not just workflow).

**Verification:**
- PR is open, green, and reviewed (Matt approves but the actual code is Trevin's).
- After merge, the workflow file no longer mentions `NPM_TOKEN`.

---

### U6. Trevin: smoke-test the OIDC release on a real tag

**Goal:** Prove the auto-release path works end-to-end before declaring the handoff done.

**Requirements:** R2

**Dependencies:** U5 merged

**Files:**
- Modify: `npm/package.json` (bump version to `0.1.1`)
- Modify: `npm/CHANGELOG.md` (add 0.1.1 entry)
- Optionally modify: `npm/README.md` (small no-op fix to give the bump a reason to exist)

**Approach:**
- Bump `npm/package.json` version to `0.1.1`. Land via PR.
- After merge, tag the merge commit: `git tag npm-v0.1.1 && git push --tags`.
- Watch the `Publish npm package` workflow run. Verify in the Actions UI that the token is provisioned via OIDC (no `NPM_TOKEN` anywhere) and the publish step succeeds.
- Verify `npm view @mvanhorn/printing-press version` returns `0.1.1`.

**Test scenarios:**
- Happy path: workflow succeeds, npm shows v0.1.1, `npx -y @mvanhorn/printing-press@0.1.1` works.
- Error path: workflow fails on the publish step with "401 Unauthorized" → trusted publisher config doesn't match (workflow filename, repo, or owner mismatch). Re-check U4 settings.
- Error path: workflow fails earlier (test or build) → not a release problem; fix forward, repeat the tag flow.

**Verification:**
- Two versions visible on npm: 0.1.0 (manual) and 0.1.1 (OIDC). Tag-driven release confirmed working.

---

### U7. Matt: revoke the temp token (optional cleanup)

**Goal:** Tidy up the unused credential. Not load-bearing — OIDC owns publishing now and the token will expire on its own at 90 days.

**Requirements:** R4

**Dependencies:** U6 verified

**Files:** None (npm UI only)

**Approach:**
- Whenever Matt happens to be in the npm UI: Profile → Access Tokens → locate the granular token from U1 → Revoke.
- If forgotten, the 90-day TTL handles it.

**Verification:**
- Token is gone from the list, or it's expired. Either is fine.

---

## System-Wide Impact

- **Interaction graph:** This plan introduces a new public distribution channel for the installer (`npx`/npm). Existing distribution paths (downloading `pp` binaries, the Claude Code plugin install) are untouched.
- **Error propagation:** A broken publish blocks future releases but doesn't degrade in-flight users — npm doesn't auto-upgrade installed packages.
- **State lifecycle risks:** v0.1.0 is forever — npm forbids overwriting a published version. U2's pre-publish validation is the only gate. Take it seriously.
- **API surface parity:** None. The npm package wraps the existing `pp` CLI; no new public API.
- **Integration coverage:** U6's real-tag smoke test is the integration check that workflow + OIDC + npm registry all line up.
- **Unchanged invariants:** Existing GitHub releases (per-CLI binaries) keep working. The `pp` CLI's behavior is unchanged. `cli-printing-press` (the generator repo) is untouched.

---

## Risks & Dependencies

| Risk | Mitigation |
|------|------------|
| Trevin's npm account doesn't exist or his username differs from `tmchow` | U1 confirms the username out-of-band before Matt runs `npm owner add`. |
| Trusted Publisher config drifts (workflow rename, repo move) | Document the trusted-publisher fields in `npm/CHANGELOG.md` or repo README so future renames know to update npm too. |
| First publish fails and burns v0.1.0 | npm rejects re-publish of the same version; bump to v0.1.1 in `npm/package.json` and republish. v0.1.0 stays as a tombstone. |
| OIDC silently regresses to needing `NPM_TOKEN` (e.g., setup-node downgrade) | Workflow no longer references `NPM_TOKEN` after U5; a regression would fail loudly with 401, not silently fall back. |
| Matt's npm username isn't `mvanhorn` and the org name is taken | Highly unlikely (he'd know by now), but U1 surfaces it on the first `npm whoami`. Fallback: rename the package or use a different scope — would invalidate Trevin's plan and need re-coordination. |

---

## Documentation / Operational Notes

- After U6 lands, add a short "Releasing" section to `npm/README.md` (or repo root README's release section if one exists) that says: "Bump `npm/package.json`, merge, then `git tag npm-v<version> && git push --tags`. The workflow handles the rest." This is a future maintainer's first hint, not a launch blocker.
- Record the trusted-publisher configuration (org `mvanhorn`, repo `printing-press-library`, workflow `npm-publish.yml`) somewhere durable — e.g., a comment block at the top of `npm-publish.yml`. If the workflow is ever renamed, npm UI must be updated to match or releases will 401.

---

## Sources & References

- **Origin document (Trevin's plan):** https://www.proofeditor.ai/d/c8v9n24h
- Existing workflow: `.github/workflows/npm-publish.yml`
- Existing package: `npm/package.json`, `npm/bin/pp.mjs`
- npm Trusted Publishing: https://docs.npmjs.com/trusted-publishers
- npm granular access tokens: https://docs.npmjs.com/about-access-tokens
