# Changelog

This file is maintained by printing-press-library release automation. Do not hand-edit release sections in normal PRs.

## Unreleased

- fix(sync): normalize CRLF line endings before parsing — a `---\r\n` frontmatter delimiter previously parsed as ZERO frontmatter keys, silently hiding the note from every frontmatter-based mirror query while it rendered fine in Obsidian (observed in production 2026-07-13: 4 pages invisible to a type-coverage lint).
- feat(sync): `--vault-path <abs>` — sync a vault directory directly, no Obsidian dependency (walker skips dot-dirs/node_modules; composes with `--folder`).
- feat(sync): per-vault mirror DBs (`vault-<name>-<hash>.db`) + `current.json` pointer; read commands follow the most recently synced vault. Fixes same-machine sibling vaults pruning each other's mirror rows and cross-vault mtime-checkpoint confusion (observed 2026-07-10: `deleted: 836` on vault switch).
- feat(sync): nested-wrapper tripwire — warns with the exact inner-folder `--vault-path` fix when the resolved vault looks like the OUTER wrapper of a `<Name>/<Name>/` layout (wrapper-rooted mirrors read ~71% of links as falsely broken with no error raised).
- sync output now includes the mirror `db_path`.

## 2026.7.1 - 2026-07-08

- fix(catalog): require Go 1.26.5 across published modules (#1467).

## 2026.6.2 - 2026-06-21

- fix(catalog): require Go 1.26.4 across published modules (#1308).

## 2026.6.1 - 2026-06-08

- Baseline release metadata added for this published CLI.
