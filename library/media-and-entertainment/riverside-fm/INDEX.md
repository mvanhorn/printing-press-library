---
name: riverside-fm-pp-cli
type: printed-cli
status: active
description: Get whichever exists for a recording in priority order: transcript first, then audio tracks, then HLS video — your stated goal as a single command.
binary_name: riverside-fm-pp-cli
binary_install_path: ./cmd/riverside-fm-pp-cli

trigger_phrases:
  - "download my riverside transcripts"
  - "grab my riverside recording"
  - "bulk export riverside studio"
  - "search riverside transcripts"
  - "harvest magic clips"
  - "use riverside-fm"
  - "run riverside-fm"
related_skills: []
printed_from: 2026-05-12 via Printing Press v4.4.0 (run 20260511-212938)
printed_by: dstevens
---

# Riverside

Get whichever exists for a recording in priority order: transcript first, then audio tracks, then HLS video — your stated goal as a single command.

## When to use

- Bulk download every transcript / audio track / video from your Riverside.com studio.
- Search Riverside transcripts (e.g., "every meeting that mentions X").
- Harvest magic clips for repurposing into other content.
- Backfill a local archive of Riverside recordings the platform doesn't make easy.

## Install

> **Note: cookie auth.** Riverside doesn't expose a public API token. Authentication is via captured browser cookies from a logged-in `riverside.fm` session. The Keychain step the generic installer offers won't fire — credentials live in a cookie store managed by the CLI itself.

```bash
# 1. Symlink + go install (no Keychain prompt — auth_env_var is empty)
bash ~/Documents/Dev/cc-skills/scripts/install.sh riverside-fm-pp-cli

# 2. Log into riverside.fm in Chrome (your normal browser session).

# 3. Import cookies (see the bundled SKILL.md `## Auth` section for the
#    canonical command; typically:)
riverside-fm-pp-cli auth login --chrome

# 4. Verify
riverside-fm-pp-cli doctor
```

If the cookie expires (Riverside rotates sessions), re-run `riverside-fm-pp-cli auth login --chrome` after logging back in.

## Invoke

```bash
riverside-fm-pp-cli --help
riverside-fm-pp-cli doctor
# Bulk-export every recording in a studio:
riverside-fm-pp-cli studios list --json
riverside-fm-pp-cli studios export <studio-id> --output ~/Documents/riverside-exports/
# Search transcripts for a phrase:
riverside-fm-pp-cli transcripts search "phrase"
```

## Trigger phrases

- `download my riverside transcripts`
- `grab my riverside recording`
- `bulk export riverside studio`
- `search riverside transcripts`
- `harvest magic clips`
- `use riverside-fm`
- `run riverside-fm`

## Related

(fill in)
