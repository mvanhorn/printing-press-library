# SculptOK CLI — Novel Features Brainstorm (subagent audit trail)

## Customer model

**Mara — CNC sign & topographic-relief maker (small shop owner)**
- Today: uploads photos one at a time to the web app, eyeballs the 3 depth-map candidates, manually downloads one, re-uploads to CAM. No record of which settings produced which result.
- Weekly ritual: 10-30 customer photos through depth-map -> pick candidate -> CNC router for carved-relief signs/plaques.
- Frustration: async submit-then-refresh dead time; loses track of which style/version/hd_fix combo carved best; re-pays credits redoing work.

**Devin — Resin/FDM hobbyist (image -> STL)**
- Today: converts pet photos/logos to lithophanes/relief plaques, guesses thickness/width, burns credits on bad STLs.
- Weekly ritual: a few image->STL jobs a week, tweaking width_mm/min_thickness/max_thickness/invert until a plate prints cleanly.
- Frustration: no way to see "what thickness settings produced the print that came out good?" Each retry costs credits; 4k/pro stings.

**Priya — Jeweler / bas-relief artist (precision + pre-processing)**
- Today: photographs models, removes background + HD-restores by hand before depth-mapping, switching tools mid-flow.
- Weekly ritual: bg-removal+HD restore (2 cr) -> depth-map pro2k/pro4k -> 3D draw for casting masters.
- Frustration: multi-step pipeline lives in her head; pro4k draws expensive; no spend visibility until wallet is low.

**Theo — Plugin/automation power user (ZBrush/Blender, batches)**
- Today: wants headless scriptable runs over folders; hand-rolls curl against the envelope.
- Weekly ritual: batch-submit dozens of images overnight, collect URLs, wire into downstream tooling, check credit burn vs output.
- Frustration: async {code,msg,data} + HTTP-always-200 makes hand-scripting submit->poll->download brittle; no local ledger to reconcile credits vs jobs.

## Candidates (pre-cut)
(see survivors/kills; full pass-2 list condensed)
1. Generate end-to-end (depthmap/stl/3d/restore) — (a)/(b) KEEP
2. Batch generate over folder — (a)/(b) KEEP
3. Credit-cost preflight estimator — (b) KEEP
4. Local job search — (c) KEEP (reframed to search --type jobs)
5. Spend analytics by kind/day — (c)/(b) KEEP
6. Result download/re-fetch (pull) — (b) KILLED
7. Settings recall ("what worked") — (c)/(a) KILLED
8. Watch pending job (status --watch) — (b) KILLED
9. Pre-process then draw (--restore-first) — (a)/(b) KEEP
10. Candidate triplet picker — (b) KILLED
11. Sync local store from history — (c) KEEP
12. Stale/stuck-job report — (c)/(b) KILLED
13. Reconcile credits vs jobs — (c) KEEP

## Survivors and kills

### Survivors
| # | Feature | Command | Score | Buildability(subagent) | Evidence | Long Description |
|---|---------|---------|-------|------------------------|----------|------------------|
| 1 | Generate (image->result end-to-end) | generate depthmap/stl/3d/restore <local-image> | 9/10 | hand-code | Brief workflows #1-4 + pain #3; build priority #3 | none |
| 2 | Batch generate over folder | generate <kind> --batch <dir> | 7/10 | hand-code | Persona Theo; "scriptable pipeline" | Use for many images of SAME kind; single image use generate |
| 3 | Credit-cost preflight estimator | cost <kind> [--version pro4k] [--batch <dir>] | 7/10 | hand-code | Pain #1 credits burn; table stakes cost transparency | none |
| 4 | Local job search | search --type jobs --limit N | 7/10 | hand-code* | Data layer FTS; competitor gap | none |
| 5 | Spend analytics by kind/day | analytics --type credit_events --group-by kind | 7/10 | hand-code* | Pain #1 + spend tracking | reports WHERE credits went; cost = forward estimate |
| 6 | Pre-process then draw | generate depthmap <img> --restore-first | 6/10 | hand-code | Persona Priya; workflow #4 | restore before draw; restore alone = generate restore |
| 7 | Reconcile credits vs jobs | reconcile --db <path> | 6/10 | hand-code* | separate credit_events+jobs entities; pain #1 | audits spend vs produced jobs |
| 8 | Sync local store from history | sync --resources jobs,credit_events,images | 6/10 | spec-emits | data layer sync cursor | none |

\* Orchestrator note: #4 search, #5 analytics, #7 reconcile, #8 sync are framework commands the generator ships once the local store/resources are declared. Genuine NEW hand-Go = the `generate` workflow family (incl. --batch, --restore-first) + `cost`. The framework commands require the custom `jobs`/`credit_events`/`images` store + sync wiring.

### Killed candidates
| Feature | Kill reason | Closest survivor |
|---------|-------------|------------------|
| pull (re-download) | generate already downloads result URLs; thin file-grab | Generate |
| recall ("what worked") | subset of search with status=success --select params | Local job search |
| draw status --watch | thin re-poll the generate workflow already owns | Generate |
| candidates triplet picker | single-field extraction covered by search/generate --select | Local job search |
| stale/stuck-job report | narrow; falls out of reconcile + search on status | Reconcile |
