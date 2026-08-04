# Research brief — Tableau agent CLI

## Problem source
Ann Jackson, “XML Hacking Tableau Workbooks with Claude” (2026-08-03)
https://annujackson.substack.com/p/xml-hacking-tableau-workbooks-with

## Findings absorbed
- Works: bulk calculated fields; basic sheets; style mimic from good example
- Fails: freeform dashboard XML; zone layout; novel chart grammar; no validate loop
- Prescription: examples + template/transform + inspect before invent

## Technical sources
- Tableau REST API docs + OpenAPI subset + Postman
- tableau/document-api-python (MIT) fixtures — connection/field focused, not full authoring
- Community .twb Superstore fixtures (Assignment_1, sales analysis)
- ranvithm/tableau.xml structure guide

## Product split
- Track A workbook compiler (this CLI core)
- Track B REST PAT bridge
- PP factory optional for REST amplify / library norms

## NOI
Tableau for agents is a validated workbook compiler + estate I/O — not raw REST and not freeform XML dumps.
