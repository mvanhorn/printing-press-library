# Extron CLI Absorb Manifest

Run: 20260811-011552-d999fbbe · Stamp: 2026-08-11-011552

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Browse literature by category + alphabetical index | Extron literature.aspx (tabid=5, id=A-Z/All; per-category tables) | extron-pp-cli literature list --category manual --letter m | Offline, `--json`, local filters, no website tabs |
| 2 | Download spec sheet / user manual PDF | Extron /download/files/<category>/<file>.pdf | extron-pp-cli literature download <url|name> --dir ./docs | Batch, `--dry-run`, revision-aware filenames, agent-friendly |
| 3 | Free-text search across literature | Extron site "Power Search" box (WAF-gated /api/v2 JSON) | (behavior in extron-pp-cli search) FTS over the synced catalog | Works offline, regex/SQL-composable, no WAF |
| 4 | Manual lookup by model | manualslib.com / manualzz.com mirrors | extron-pp-cli literature get <model> | Official Extron PDF with Rev/Date metadata instead of third-party mirrors |

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Buildability | How It Works | Evidence | Long Description |
|---|---------|---------|-------|--------------|--------------|----------|------------------|
| 1 | Revision drift check | `literature updates` | 9/10 | hand-code | Joins the local downloaded-files table (revision parsed from revision-aware filenames) against the synced catalog's Rev column in local SQLite and lists docs where local_rev < catalog_rev | Brief: revision-aware download filenames + local-catalog-with-revision-metadata thesis; integrators keep stale PDFs with no drift detection | Use this to find which downloaded docs need re-downloading. Do NOT use it to browse brand-new library arrivals; use 'literature recent' instead. |
| 2 | Doc-set completeness | `catalog completeness` | 8/10 | hand-code | Groups the synced catalog by model and cross-tabs the six categories (Brochure / DoC / Design Guide / Product Guide / Manual / Revit BIM) to report which doc types each model in your downloads (or a --bom list) is missing — a pure local SQLite query | Brief Users (quoting/compliance); the six literature.aspx categories are a service-specific content pattern | Use this to find which doc types are missing per model for bids/commissioning. Do NOT use it for a single model's doc list; use 'literature get' instead. |
| 3 | Library what's-new | `literature recent` | 7/10 | hand-code | Queries the synced catalog ordered by the Date column with --days/--category filters to list the newest literature across the whole library, fully offline | API spec: the literature.aspx table exposes a Date column; firmware/manual update tracking is an integrator workflow | Use this to track what Extron released recently across the library. Do NOT use it for the drift of your own downloads; use 'literature updates' instead. |
| 4 | BOM doc bundle | `literature rack --bom <file>` | 7/10 | hand-code | Reads a rack BOM file of model numbers and resolves each model through the local catalog, then reports or batch-downloads each model's full doc set | Task context: rack builds/quoting are the operative integrator workflow; brief Top Workflows are fleet/project-shaped batch operations | Use this to assemble doc sets for an entire rack at once. Do NOT use it for a single model; use 'literature get' instead. |
| 5 | Series grouping | `literature family <series>` | 6/10 | hand-code | Matches catalog Description against a small curated list of mechanical family prefixes (DTP, MAV, IPL, DVS, PCS, ...) to group all docs for one product series — local SQLite string-prefix match, no NLP | API spec: catalog titles embed model names and are indexed only by first letter on the site; brief device families (MAV/DTP/DVS) are the prefix vocabulary | Use this to browse every doc for a product family across letters. Do NOT use it for a single model's official docs; use 'literature get' instead. |
| 6 | Download integrity | `catalog verify` | 6/10 | hand-code | Joins local PDF file sizes and filename revisions against the catalog's Size and Rev columns to flag mismatches or truncated downloads — local data only | API spec: Size/Rev metadata per row plus WAF-gated transport that makes downloads the flaky leg; brief's dry-run/batch download makes integrity a follow-on need | none |

## Source Priority
- Not a combo CLI; single source: extron.com literature library.

## Scope Notes
- The CLI targets the Extron **literature library** (spec sheets + user manuals) per the user's explicit choice. Extron **device control** (SIS/TCP, the protocol researched in the brief) is out of scope for this print and belongs in anti-triggers.
