---
name: pp-tableau
description: "Use when working with Tableau workbooks (.twb/.twbx) or Tableau Server/Cloud REST as an agent. Prefer structured CLI ops over freeform XML."
created_by: user
version: 0.3.0
---

# Tableau agent skill (workbook + REST)

## Install / build

```bash
# Workbook compiler (Ann fix)
cd workbook && go build -o bin/tableau-workbook-pp-cli ./cmd/tableau-workbook-pp-cli
# REST bridge
cd ../rest && go build -o bin/tableau-rest-pp-cli ./cmd/tableau-rest-pp-cli
```

Put both on `PATH`.

## CRITICAL rules

1. **Never invent dashboard XML.** Use `dashboard scaffold --template …` or `dashboard clone` only.
2. **Always lint** before asking a human to open Desktop: `workbook lint` or `open-check`.
3. **Mutations** need `--output` or `--in-place`. Writes are lint-gated.
4. **Do not log** `TABLEAU_PAT_SECRET`.

## Workbook workflows

### Inspect
```bash
tableau-workbook-pp-cli workbook inspect path.twb --json
tableau-workbook-pp-cli calc list path.twb --json
tableau-workbook-pp-cli sheet list path.twb --json
tableau-workbook-pp-cli dashboard list path.twb --json
```

### Bulk calcs (Ann path)
```bash
# One-shot CY/PY/Delta/YoY% for measures
tableau-workbook-pp-cli calc pack-yoy path.twb \
  --measures Sales,Profit,Quantity \
  --date-field "Order Date" --cy-year 2017 --py-year 2016 \
  --output out.twb

# Or JSON batch
tableau-workbook-pp-cli calc add-batch path.twb --file calcs.json --output out.twb
```

### Sheet + style mimic (what worked for Ann)
```bash
tableau-workbook-pp-cli sheet clone path.twb --from "Good Sheet" --to "New Sheet" --output out.twb
tableau-workbook-pp-cli style apply out.twb --from "Good Sheet" --to "New Sheet" --in-place
```

### Dashboards (template-only)
```bash
tableau-workbook-pp-cli dashboard templates --json
tableau-workbook-pp-cli dashboard scaffold path.twb \
  --name "Ops Board" --template two-pane \
  --sheets "Sheet A,Sheet B" --output out.twb
# templates: single | two-pane | three-row | quad
```

### Validate
```bash
tableau-workbook-pp-cli workbook lint out.twb
tableau-workbook-pp-cli open-check out.twb
```

## REST workflows

```bash
export TABLEAU_SERVER=https://POD.online.tableau.com
export TABLEAU_SITE_CONTENT_URL=   # empty default site
export TABLEAU_PAT_NAME=...
export TABLEAU_PAT_SECRET=...

tableau-rest-pp-cli auth login --json
tableau-rest-pp-cli workbooks list --json
tableau-rest-pp-cli workbooks download --id LUID --output wb.twbx
# edit with workbook CLI...
tableau-rest-pp-cli workbooks publish --file out.twb --project-id PID --name "Name" --dry-run
```

## Full agent loop
1. `workbooks download`
2. `workbook inspect`
3. `calc pack-yoy` / sheet clone / style apply / dashboard scaffold
4. `workbook lint` (must be ok)
5. `workbooks publish`

## When stuck
- Desktop error about enums / content model → run `lint`; fix via structured ops, not hand XML.
- Unknown chart grammar → clone a known-good sheet, don't invent marks XML.
- No PAT → workbook-only offline with fixtures under `testdata/`.
