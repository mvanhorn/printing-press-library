# Phase 4 Shipcheck — passive-indices-pp-cli

`cli-printing-press shipcheck --dir "$CLI_WORK_DIR" --spec "$RUN/research/passive-indices-spec.yaml" --research-dir "$RUN"`

## Result: PASS (7/7 legs), after one fix

First run FAILED `validate-narrative`: the quickstart example
`sync --resources indices,funds` errored with `unknown sync resource
"indices"`. Root cause: this API only has one sync resource, `"index"`
(singular, niftyindices' live snapshot) — there is no bulk `"funds"`
resource at all; indiapassivefunds data is fetched live per-command, never
bulk-synced. Fixed the quickstart in `research.json` and `README.md` to
`sync --resources index` with an accurate comment. Re-ran: PASS 7/7.

| Leg | Result |
|---|---|
| verify | PASS (41/41, 100%) |
| validate-narrative | PASS (after fix above) |
| dogfood | PASS (9/9 novel features, WARN only on `defaultSyncResources empty`, an accepted design characteristic) |
| workflow-verify | PASS (no workflow manifest, skips cleanly) |
| apify-audit | PASS (no Apify actor references) |
| verify-skill | PASS (0 findings, canonical-sections passed) |
| scorecard | 76/100 (Grade B) |

## Doc audit (README.md / SKILL.md)

Swept for stale "weight"/`topAUM`/`channels,messages` residue from earlier
fixes — clean. Fixed one generic troubleshooting line in README.md ("Run the
`list` command to see available items" — this CLI has no `list` command;
replaced with `index live` / `fund search` pointers).

## Retro candidates found (machine-level, not fixed here)

1. `sync.go.tmpl` / `graphql_sync.go.tmpl` (`internal/generator/templates/`):
   hardcoded generic example `sync --resources channels,messages` in the
   generated `--help` output, not derived from the actual spec's resource
   names.
2. `readme.md.tmpl:652-653`: hardcoded "Run the `list` command to see
   available items" troubleshooting line, assumes every generated CLI has a
   `list` command.

Both are cosmetic (don't break anything) and out of scope for a single
printed-CLI run — flagged for whoever owns generator template maintenance.
