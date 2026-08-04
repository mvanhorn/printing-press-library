# tableau-pp — agent-native Tableau toolkit

**Genuine fix for [Ann Jackson’s agent + Tableau workbook XML experiment](https://annujackson.substack.com/p/xml-hacking-tableau-workbooks-with)**  
plus a REST bridge for Cloud/Server estate I/O.

Publish target: [printing-press-library](https://github.com/mvanhorn/printing-press-library)  
Generator machine (optional for REST amplify): [cli-printing-press](https://github.com/mvanhorn/cli-printing-press)

## Two binaries

| Binary | Problem | Path |
|--------|---------|------|
| `tableau-workbook-pp-cli` | Local `.twb` compiler: parse → structured ops → **lint** → write | `workbook/` |
| `tableau-rest-pp-cli` | PAT auth, list, download, publish | `rest/` |

## Why not Printing Press alone?

PP is excellent for **HTTP APIs**. Ann’s failure was an **undocumented file format** (illegal dashboard XML, invented enums). Track A is hand-built for that. Track B is REST and can grow with PP.

## Quick start

```bash
# Workbook (Ann fix)
cd workbook && go test ./... && go build -o bin/tableau-workbook-pp-cli ./cmd/tableau-workbook-pp-cli

./bin/tableau-workbook-pp-cli workbook inspect testdata/superstore/Assignment_1.twb --json
./bin/tableau-workbook-pp-cli calc pack-yoy testdata/official/sample-superstore.twb \
  --measures Sales,Profit,Quantity --output /tmp/out.twb
./bin/tableau-workbook-pp-cli dashboard scaffold /tmp/out.twb \
  --name "Ops" --template two-pane --sheets "Sheet 1,Sheet 1" --dry-run
# (use real distinct sheets from inspect)

./bin/tableau-workbook-pp-cli open-check /tmp/out.twb

# REST bridge
cd ../rest && go test ./... && go build -o bin/tableau-rest-pp-cli ./cmd/tableau-rest-pp-cli
export TABLEAU_SERVER=https://YOURPOD.online.tableau.com
export TABLEAU_PAT_NAME=...
export TABLEAU_PAT_SECRET=...
./bin/tableau-rest-pp-cli auth login --json
./bin/tableau-rest-pp-cli workbooks download --id <LUID> --output ./wb.twbx
```

## Agent loop

```
rest download → workbook calc pack-yoy / sheet clone / style apply / dashboard scaffold
  → workbook lint → rest publish
```

## Ann success criteria (met in tests)

1. Bulk calcs without hand XML (`pack-yoy`, `add-batch`) ✅  
2. Style clone across sheets ✅  
3. Dashboard only via known-good templates or clone ✅  
4. Lint rejects illegal `bold` enum and missing `simple-id` ✅  
5. Write refused when lint fails ✅  

## Library packaging

See `library/developer-tools/tableau/` for printing-press-library shaped tree  
(`.printing-press.json`, `SKILL.md`, manuscripts, goreleaser stubs).

## License

MIT (project). Fixture attributions in `fixtures/SOURCES.md` and workbook README.
