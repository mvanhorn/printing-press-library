# Proofs — offline shipcheck (2026-08-03)

## Workbook CLI
```
cd workbook && go test ./...   # PASS
go build -o bin/tableau-workbook-pp-cli ./cmd/tableau-workbook-pp-cli
```

Covered:
- Inspect Assignment_1: 6 sheets, 1 dashboard, 38 zones
- Lint clean on sample-superstore + Assignment_1
- AddCalc + AddCalcs batch 36
- pack-yoy 12 calcs (Sales/Profit/Quantity)
- CloneSheet, ApplyStyle
- Scaffold two-pane + three-row
- Reject unknown template
- Lint reject bold enum + missing simple-id
- Open twbx
- Write lint gate

## REST CLI
```
cd rest && go test ./...       # PASS (offline XML fixtures)
go build -o bin/tableau-rest-pp-cli ./cmd/tableau-rest-pp-cli
workbooks publish --dry-run    # plan prints without network upload when dry-run
```

## Live
- REST live dogfood: blocked until TABLEAU_PAT_* provided
- Desktop open-check: no Tableau.app on build machine

## PP machine
cli-printing-press 4.30.1 installed via go install
