# prediction-goat playbooks

Per-CLI playbook content for `prediction-goat-pp-cli`. Each playbook is a JSON file (CLI choreography) plus a `_notes.md` file (free-text gotchas / workarounds). Both ship as embedded data and auto-install into the `learning_playbooks` table at first DB open.

## Adding a playbook

1. Create `<query_family_name>.json` matching the `learn.Playbook` shape.
2. Create matching `<query_family_name>_notes.md` with workarounds.
3. Bump `SeedVersion` in `embed.go` so existing installs re-seed.

## Convention

- One playbook per query family. The install path normalizes each `query_family_examples` entry via `learn.Normalize` + `learn.QueryFamily` and upserts under every distinct family.
- Notes are markdown; the agent reads them verbatim per SKILL.md.
- This `MANIFEST.md` stub keeps `//go:embed *.md` matching at least one file. Do not delete it.
- The embed pattern is currently `*.md` only because U9 ships with no hand-authored JSONs yet. U10 adds the JSON files and bumps the pattern to `*.json *.md`.

## Cross-CLI port

To add this pattern to another PP CLI:
- Copy this directory's `embed.go` (rename SeedVersion to the new CLI).
- Copy `internal/cli/playbook_init.go` from this library.
- Add `runPlaybookInitOnce(cmd.Context())` to the new CLI's PersistentPreRunE alongside any existing learn-init.
- Author the per-CLI JSON+MD files in the new playbooks directory.
