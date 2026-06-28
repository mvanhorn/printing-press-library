---
title: Live LinkedIn Apply CLI UX
status: executing
execution: code
created: 2026-06-27
repo: /home/mgmarcusa/profilepress-printed
---

# Live LinkedIn Apply CLI UX

## Product Bar

ProfilePress should feel like a real CLI for a mainstream web app account, not a browser steerer.

Normal UX:

```bash
profilepress snapshot --chrome-session --profile-url <linkedin-url>
profilepress propose-for-job --snapshot <id> ...
profilepress diff --packet <id>
profilepress apply-packet --packet <id> --live-linkedin --confirm-sensitive APPLY-SENSITIVE --confirm-apply APPLY
```

The user should not need to:

- copy/paste profile text;
- relogin through a separate browser;
- launch Chrome with debugging flags;
- understand CDP, ports, browser window IDs, or DOM selectors;
- have their active Chrome closed/relaunched/disrupted.

## Scope Boundaries

In scope:

- LinkedIn-specific apply adapter, not generic browser steering.
- Use existing local Chrome LinkedIn session import, same ambient model as `snapshot --chrome-session`.
- Map ProfilePress packet sections to LinkedIn-specific write operations.
- At minimum support top-card/headline and About, because these have structured profile API data and are safer to update first.
- Treat full Experience editing as supported only if a LinkedIn-specific write endpoint and stable entity identifiers are discovered and verified; otherwise fail closed with a precise unsupported-section error rather than pretending to apply.
- Keep all sensitive writes behind confirmation gates.
- Default to preview/dry-run; live writes require `--live-linkedin`, `--confirm-apply APPLY`, and sensitive confirmation.
- Do not print session cookies, CSRF values, or other auth material.

Out of scope for this unit:

- Silent browser UI automation as the primary implementation.
- Circumventing LinkedIn auth or storing passwords.
- Editing arbitrary websites.
- Sending network notifications without explicit `--notify-network` and `--confirm-notify NOTIFY-NETWORK`.

## Implementation Units

### U1: Endpoint discovery and writer boundary

Goal: identify whether LinkedIn internal authenticated APIs can update headline/About directly from the imported Chrome session.

Files:

- `internal/browser/chrome_session.go`
- new `internal/linkedin/live_apply.go`
- new tests under `internal/linkedin` / `internal/cli`

Verification:

- Failing test before production code for adapter selection and unsupported sections.
- Discovery notes in this plan or docs if an endpoint cannot be safely confirmed.

### U2: CLI apply UX

Goal: make `apply-packet` expose a real LinkedIn live mode.

Expected UX:

```bash
profilepress apply-packet \
  --packet pkt_... \
  --live-linkedin \
  --privacy-status disabled \
  --confirm-sensitive APPLY-SENSITIVE \
  --confirm-apply APPLY
```

Behavior:

- Without `--live-linkedin`, no real LinkedIn write occurs.
- With `--live-linkedin`, use imported Chrome session adapter.
- Unsupported sections fail closed before partial writes unless user explicitly scopes sections in a later design.
- Result is `linkedin-apply-passed` only after LinkedIn-specific write adapter reports success.

### U3: Safety and verification

Goal: prove behavior without accidentally changing the user's live profile during tests.

Verification:

- Unit tests use a fake LinkedIn HTTP server/session helper.
- Local build/test/publish validate pass.
- Ad-hoc verifier under `/tmp/hermes-verify-*` confirms CLI wiring and dry-run/fake-server behavior.
- Any actual live profile write requires explicit user confirmation in a separate turn after a preview.

## Discovery Results

Read-only discovery found LinkedIn-specific SDUI save contracts:

- Top-card/headline request ID: `com.linkedin.sdui.requests.profile.saveProfileIntroForm`
- About request ID: `com.linkedin.sdui.requests.profile.saveProfileAboutForm`
- Position request ID: `com.linkedin.sdui.requests.profile.saveProfilePositionForm`
- Transport path: `<edit-page-como-ep>/rsc-action/actions/server-request?sduiid=<requestId>`

The current implementation supports `headline` and `about`. `experience` is intentionally rejected because ProfilePress currently represents Experience as one monolithic section, while LinkedIn writes individual position forms with per-position IDs and form payloads. A proper Experience writer needs a position-level packet model first.

## Risks

- LinkedIn internal write APIs may be unstable or CSRF/version-sensitive.
- Updating experience entries may require stable entity IDs not present in current snapshot text.
- Writing via internal APIs can partially update if not designed carefully; adapter must validate all sections before first write.
- Privacy/broadcast setting may not be safely machine-verifiable yet; keep user-provided `--privacy-status disabled` gate for now.

## Success Criteria

- `apply-packet --live-linkedin` is a LinkedIn-specific CLI command path, not a generic browser steerer.
- The command uses local Chrome session import without disrupting Chrome.
- The command either performs supported LinkedIn writes or fails closed with actionable unsupported-section errors.
- Source repo and Printing Press PR are updated and green.
