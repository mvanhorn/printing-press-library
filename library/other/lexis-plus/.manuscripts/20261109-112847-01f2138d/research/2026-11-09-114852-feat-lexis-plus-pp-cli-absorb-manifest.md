# Lexis+ Absorb Manifest (user-approved 2026-11-09)

## Absorbed
| # | Feature | Best Source | Our Implementation |
|---|---------|-------------|--------------------|
| 1 | Get case by citation | Lexis+ search box | (generated endpoint) lookup start + lookup result |
| 2 | Get CA statute by citation | Lexis+ search box | (generated endpoint) lookup start + lookup result |
| 3 | Download as Word (case options) | Lexis+ Download dialog | (generated endpoint) delivery create + delivery status + delivery get + delivery file |
| 4 | Download statute with annotations | Lexis+ Download dialog | (behavior in lexis-plus-pp-cli delivery create, retrieveoptions.includeAnnotations=true) |
| 5 | Citation type-ahead | Lexis+ wordwheel | (generated endpoint) lookup suggest |

## Transcendence (approved)
| # | Feature | Command | Buildability | Score | Persona |
|---|---------|---------|--------------|-------|---------|
| 1 | Brief cite extractor | brief cites <file> | hand-code | 9 | Maya, Rosa |
| 2 | Brief cite verifier | brief check <file> | hand-code | 8 | Maya |
| 3 | Brief gaps vs pulled log | brief gaps <file> | hand-code | 8 | Maya, Rosa |
| 4 | Matter ledger | matter list/show <name> | hand-code | 7 | Rosa, Dev |
| 5 | Statute annotation diff | statute whatsnew <cite> | hand-code | 7 | Dev |
| 6 | Authority binder | matter binder <name> --out <dir> | hand-code | 6 | Rosa |

## Killed
Parallel-cite lookup, log search (duplicates framework search), have (covered by batch skip-existing), case signals drift (breaches on-demand constraint), statute range expander (no listed endpoint), brief TOA (no page numbers), pinpoint extractor (no endpoint for page boundaries).
