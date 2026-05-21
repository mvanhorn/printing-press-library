# Novel Features Brainstorm (Phase 1.5c.5 audit trail)

## Customer model

### Persona 1: Tereza, 34, Prague desk-worker on a 90 kg → 75 kg cut

**Today (without this CLI):** Tereza opens kaloricketabulky.cz in a browser tab every meal — she types "tvaroh nízkotučný", waits for the AngularJS autocomplete to populate, clicks the item, manually picks the right meal slot (Snídaně/Svačina/Oběd/Svačina/Večeře) from a dropdown, types the grams, hits save. Five clicks per food, six foods a day, plus the weight modal every morning. She cannot answer: "what foods have I logged most over the last 30 days?", "am I systematically short on protein at dinner?", "how does my actual energy intake compare to my Harris-Benedict target across the week?". The web UI shows one day at a time.

**Weekly ritual:** Sunday evening she opens the PDF export per day for the past 7 days, eyeballs the macro bars, scribbles a target adjustment in her notebook. Monday morning she weighs in. Throughout the week she logs every meal in the diary mid-meal on her phone or laptop, switches between meal slots, copies yesterday's breakfast when it's the same oats. She uses copy-day for travel days when she eats the same thing as the day before.

**Frustration:** Click-cost of logging while cooking. The diary refuses cross-day analytics — the only way to see "what's my actual 30-day protein average" is 30 PDF exports. She knows she's protein-short at dinner but can't prove it without exporting and importing into a spreadsheet.

### Persona 2: Martin, 28, recreational ultrarunner training for a 100 km race

**Today (without this CLI):** Martin tracks both food and activity. He runs 80-120 km/week and his fueling has to match. He logs runs as activity entries (kcal/min × duration), then back-fills food. He cannot answer in the web UI: "given today's training load, what's my net energy balance?", "show me a 14-day moving average of energy in vs out", "find me Czech foods >20 g protein per 100 g that I've actually eaten in the last 60 days". The activity catalog has kcal/min but the web UI doesn't combine it with diary in any meaningful aggregation.

**Weekly ritual:** After each long run he logs the activity (duration, kcal estimate). Twice weekly he weighs in. Sunday he reviews the week, manually summing energy in vs out from the daily summaries. He maintains a separate Google Sheet because the site won't.

**Frustration:** No energy-balance view. No "what did I eat on long-run days vs rest days". No way to surface foods he'd forgotten that hit a macro target. Activity kcal estimates are baked into separate API calls, never joined with diary on output.

### Persona 3: Lenka, 41, nutritionist coaching ~30 paying clients

**Today (without this CLI):** Clients share screenshots of their diary; Lenka transcribes into her own spreadsheet. She uses kaloricketabulky.cz because her Czech-speaking clients use it natively (diacritics, brand-name Czech foods like "Olma", "Madeta"). She cannot pull a client's diary as data; she cannot diff two clients' weekly patterns; she cannot answer "what foods do my best-progressing clients eat in common?".

**Weekly ritual:** Monday she asks each client to PDF-export their week. She manually reads the PDFs, types macro totals into her sheet, marks compliance, writes a Tuesday-morning feedback note per client.

**Frustration:** PDF is a write-only format. She wants `--json` so she can pipe a client's diary into pandas. She wants weight regression with a slope so she can show clients "you're losing 0.4 kg/week on average". She wants frequency analysis ("you ate bread 18 of 30 days") to drive coaching conversations. None of this is in the web UI.

## Candidates (pre-cut)

### (a) Persona-driven
1. **`diary log <food-query> --grams N --meal lunch`** — fuzzy-search foods locally (FTS5), log in one command. **Keep.**
2. **`macros gap [--date today|--days 7]`** — target - actual per macro across window. **Keep.**
3. **`energy balance [--days 14]`** — diary energy in minus activity energy out. **Keep.**
4. **`weight trend [--days 90]`** — Absorbed table #13. **Skip.**
5. **`diary frequency [--days 30] [--meal SLOT] [--min N]`** — foodstuff occurrence count. **Keep.**

### (b) Service-specific content patterns
6. **`food substitutes <id> [--by macro]`** — macro-distance similar foods. **Keep.**
7. **`kj kcal <value>`** — trivial conversion. **Cut.**
8. **`food rich-in <nutrient>`** — sibling-killed by 6. **Cut.**
9. **`meal-slot distribution`** — sibling-killed by 2. **Cut.**
10. **`food allergens <id>`** — JSON-LD keyword mining. **Keep.**

### (c) Cross-entity local queries
11. **`diary export json --from --to`** — bulk JSON. **Keep.**
12. **`day compare <a> <b>`** — sibling-killed by 2. **Cut.**
13. **`diary plan-meal --target-protein N`** — greedy-select favorites/frequents. **Keep.**
14. **`weight regression`** — OLS + projection. **Keep.**

### (e) User Vision
15. **`diary unlog --last`** — undo last diary entry. **Keep.**

### (f) Codebase Intelligence
16. **`session check`** — fold into absorbed `auth refresh`. **Cut.**

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Buildability | How It Works | Evidence |
|---|---------|---------|-------|--------------|--------------|----------|
| 1 | One-command food logging | `diary log <food-query> --grams N --meal SLOT [--date]` | 9/10 | hand-code | Resolves food-query against local FTS5 cache (top hit, or `--pick` if ambiguous), then POSTs `/user/diary/foodstuff/add` | Brief Top Workflow #2, persona Tereza's click-cost; absorbed table only exposes ID-based add |
| 2 | Macro target gap across window | `macros gap [--days N] [--by meal]` | 8/10 | hand-code | Reads cached daily summaries + diary for window, computes `target - actual` per macro, groups by meal slot | Brief Product Thesis ("macro-target gap analysis"); persona Tereza |
| 3 | Energy-in vs energy-out balance | `energy balance [--days N]` | 7/10 | hand-code | Joins per-day diary energy with activity energy from local store; daily series + moving average | Persona Martin's net-balance question |
| 4 | Diary frequency analysis | `diary frequency [--days N] [--meal SLOT] [--min N]` | 7/10 | hand-code | `SELECT foodstuff_id, COUNT(*) FROM diary_entry WHERE date >= ?` over cached store | Persona Lenka; brief lists "frequency-of-foods" as transcendence target |
| 5 | Macro-similar food substitutes | `food substitutes <id> [--by protein\|carb\|fat\|energy]` | 7/10 | hand-code | Euclidean distance over typed nutrition struct from JSON-LD | Brief Product Thesis ("food substitution by macro distance") |
| 6 | Allergen mining from JSON-LD | `food allergens <foodstuff-id\|slug>` | 6/10 | hand-code | Fetches `/potraviny/<slug>`, extracts JSON-LD `keywords`, regex-matches Czech allergen tokens (lepek, laktóza, vejce, ořechy, sója, ryby) | Brief Product Thesis ("allergen mining from JSON-LD keywords") |
| 7 | Plan a meal to hit protein target | `diary plan-meal --target-protein N [--remaining-energy K] [--meal SLOT]` | 7/10 | hand-code | Reads today's summary; greedy-selects from favorites + frequents whose per-100g macros close the protein gap within energy budget | Brief Product Thesis ("what should I eat to hit my protein target tonight"); Martin |
| 8 | Weight linear regression + projection | `weight regression [--days N] [--target-kg K]` | 6/10 | hand-code | OLS over `monthWeight[]`; slope + R^2 + days-to-target | Persona Lenka |
| 9 | Bulk diary export to JSON | `diary export json --from <date> --to <date>` | 6/10 | hand-code | Iterates cached daily diary, rolls into one JSON with typed totals | Persona Lenka ("PDF is write-only") |
| 10 | Undo last diary entry | `diary unlog --last [--meal SLOT]` | 5/10 | hand-code | Reads most-recent cached entry id, calls `/user/diary/foodstuff/delete/<id>` | User Vision explicit ask |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| `weight trend` | Already absorbed (#13). | `weight regression` |
| `kj kcal <value>` | Wrapper-shaped, trivial. | `food substitutes` |
| `meal-slot distribution` | Sibling-killed by `macros gap --by meal`. | `macros gap --by meal` |
| `food rich-in <nutrient>` | Sibling-killed by `food substitutes`. | `food substitutes` |
| `day compare` | Weak weekly use; sibling-killed by windowed aggregates. | `macros gap` |
| `session check` | Sibling-killed by absorbed `auth refresh`. | `auth refresh` (absorbed) |
