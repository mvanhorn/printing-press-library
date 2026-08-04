# Tableau CLI Agent Guide

This directory ships `tableau-workbook-pp-cli` (local `.twb` compiler) and
`tableau-rest-pp-cli` (PAT REST bridge). It was **hand-built** for the Ann Jackson
workbook-XML failure modes, then packaged to Printing Press library conventions.

Systemic REST-wrapper improvements may still belong upstream in CLI Printing Press;
workbook XML correctness belongs in this tree.

## Local operating contract

```bash
tableau-workbook-pp-cli --help
tableau-workbook-pp-cli workbook inspect path.twb --json
tableau-workbook-pp-cli workbook lint path.twb
tableau-rest-pp-cli --help
```

### Hard rules for agents

1. **Never invent dashboard XML.** Use `dashboard scaffold --template …` or `dashboard clone`.
2. **Always lint** before Desktop open or publish.
3. Mutations require `--output` or `--in-place`. Writes are lint-gated.
4. **HTTPS only** for REST (`TABLEAU_SERVER` must be `https://…`).
5. Opening a **`.twbx` for mutation** that writes `.twb` drops packaged extracts/assets unless you pass `--allow-drop-package` after understanding the risk. Prefer download → work on `.twb` when possible, or keep originals.
6. Never log `TABLEAU_PAT_SECRET`.

### Common flows

```bash
# Bulk calcs (Ann path)
tableau-workbook-pp-cli calc pack-yoy path.twb --measures Sales,Profit,Quantity --output out.twb

# Style mimic
tableau-workbook-pp-cli sheet clone path.twb --from "Good" --to "Copy" --output out.twb
tableau-workbook-pp-cli style apply out.twb --from "Good" --to "Copy" --in-place

# Template dashboard
tableau-workbook-pp-cli dashboard templates
tableau-workbook-pp-cli dashboard scaffold path.twb --name Ops --template two-pane \
  --sheets "A,B" --output out.twb

# REST round-trip
export TABLEAU_SERVER=https://YOURPOD.online.tableau.com
export TABLEAU_PAT_NAME=...
export TABLEAU_PAT_SECRET=...
tableau-rest-pp-cli workbooks download --id LUID --output wb.twbx
```

See `README.md` and `SKILL.md` for full command maps.

## Local customizations

Record intentional hand-edits under `.printing-press-patches/` so future reprints
preserve intent.
