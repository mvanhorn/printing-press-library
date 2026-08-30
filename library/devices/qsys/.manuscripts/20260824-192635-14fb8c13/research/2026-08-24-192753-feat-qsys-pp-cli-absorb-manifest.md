# Q-SYS Absorb Manifest (reprint 2026-08-24)

## Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Browse product specs | www.qsys.com product pages | (generated endpoint) product page | Offline, joined with config+wiring, spec text searchable |
| 2 | List spec-sheet/manual PDFs | www.qsys.com product resources | (generated endpoint) product resources | Bulk, scriptable, no click-through |
| 3 | Read a config/networking help page | help.qsys.com | (generated endpoint) page get | Offline, version-pinned to installed Designer release |
| 4 | Hardware compatibility by Designer version | help.qsys.com compat matrix | qsys-pp-cli compat by-version | Takes a list, not one model at a time |
| 5 | Deprecation notices | help.qsys.com Deprecation_Notices.htm | qsys-pp-cli compat deprecations | Joined to an equipment list |
| 6 | Firmware/Designer upgrade path | help.qsys.com | qsys-pp-cli compat upgrade-path | Offline |
| 7 | Search documentation | site search on each of 3 sites | qsys-pp-cli search | One FTS index across all three sources, offline |
| 8 | Live Core control (QRC/Lua) | tomsfaire/q-sys-mcp, qsys-tools/qrc-client-js | (stub) DELIBERATELY NOT BUILT — user is an AV integrator, not a control programmer; no Core reachable to verify against | n/a |
| 9 | Design-file revisioning | mckay115/QSC-QSYS-Launcher | (stub) DELIBERATELY NOT BUILT — different job entirely | n/a |

## Transcendence (only possible with our approach)
| # | Feature | Command | Score | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|-------|--------------|-------------------------|------------------|
| 1 | Unified product card | product get | 10/10 | hand-code | Four-way join: product row + extracted PDF spec text + help config/wiring pages + support gotchas/reset/fault history | Use for everything about ONE model. Do NOT use for a list; use 'bom verify'. Do NOT use for a Designer fault string; use 'fault'. |
| 2 | BOM pre-quote sweep | bom verify | 10/10 | hand-code | List-in join across products, compat matrix, deprecations, LTS known-issues; emits version support + EOL + LTS carry + spec_pdf_url | Use for the full pre-quote report on an equipment list. Do NOT use for one model; use 'product get'. Do NOT use for known-issue articles; use 'bom risks'. |
| 3 | BOM known-issue sweep | bom risks | 9/10 | hand-code | NEW — only possible now that support.qsys.com is indexed; dedupes troubleshooting/awareness/error/known-issue hits across a list and filters by Designer release | Use to surface known issues across a whole equipment list. Do NOT use for the supported/EOL verdict; use 'bom verify'. Do NOT use for one model; use 'product get'. |
| 4 | Version-support verdict for a list | compat check | 10/10 | hand-code | Compat matrix parsed into rows so an arbitrary list checks against one Designer version in one pass | Use for the fast supported/not-supported verdict on a list. Do NOT use for the full report with EOL and spec columns; use 'bom verify'. |
| 5 | Designer release brief | qds | 9/10 | hand-code | Joins per-release known-issues (incl. 6-article LTS set) to compat matrix + awareness notices | Use to learn what is true of one Designer release. Do NOT use to check specific models; use 'compat check'. |
| 6 | Designer fault-string lookup | fault | 9/10 | hand-code | NEW — 38 error/status articles are titled with the literal strings Designer displays; normalizes punctuation/casing and matches title+body | Use when Designer shows a fault string and you need the fixing article. Do NOT use for general search; use 'search'. Do NOT use for wiring; use 'connect'. |
| 7 | Connection guidance by model | connect | 9/10 | hand-code | Model -> family -> filtered networking/wiring pages + 374 support application notes | Use for how-do-I-wire-this-in questions on one model. Do NOT use for the full record; use 'product get'. Do NOT use for a fault string; use 'fault'. |
| 8 | Corpus extraction depth report | coverage | 8/10 | hand-code | Reports PDFs LINKED vs PDFs TEXT-EXTRACTED per source — the prior print shipped coverage and still let the spec-text gap through because it counted the wrong number | none |

## Reprint verdicts (prior 8 features)
| Prior command | Verdict | Reason |
|---------------|---------|--------|
| product get | KEEP | 10/10; scope grows from 2-site to 3-site join |
| compat check | KEEP | 10/10; brief Top Workflow #2 verbatim |
| bom verify | KEEP | 10/10; extended with LTS carry + spec_pdf_url |
| connect | KEEP | 9/10; corpus broadens with 374 application notes |
| coverage | REFRAME (name kept) | Prior counted PDFs linked, not extracted — that is why the spec gap escaped |
| page get --version | REFRAME -> qds | Version-pinned page get is now an absorbed generated endpoint; release-level is what stays transcendent |
| compat deprecated | DROP | Subsumed by bom verify EOL column + absorbed compat deprecations |
| integrations | DROP | UC platform is one vendor axis among many once 374 app notes are indexed |

## Killed candidates (8)
compat deprecated, integrations, reset, gotchas, license, submittal, asset-manager, roomsuite faq — each folded into a survivor or judged flab. Full kill reasons in the subagent transcript.
