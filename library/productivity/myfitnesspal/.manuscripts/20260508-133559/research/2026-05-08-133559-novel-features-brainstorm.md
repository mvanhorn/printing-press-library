# MyFitnessPal CLI — Novel Features Brainstorm

This is the audit trail from the Phase 1.5 Step 1.5c.5 subagent run. The customer model and killed candidates are not folded into the manifest but are persisted here for retro/dogfood debugging.

## Customer model

### Persona 1: Maya, the macro-tracking athlete (cyclical bulker)

**Today (without this CLI):** Maya logs every meal in MFP from her phone, then on Sunday opens MFP web to eyeball the week. To actually see *per-food* protein/carb/fat trends across a 12-week bulk, she pays $80/yr for premium and exports CSV — except premium CSV rolls up to *meal* level, which collapses her 4-food breakfasts into one row. She has a Reddit-found ImportXML Google Sheet that breaks every six weeks when MFP redesigns, plus a personal Notion page where she manually re-types "what I ate when I hit 200g protein" examples. She cannot answer: "across the last 60 days, which 5 foods drove 80% of my protein?" without staring at the diary day-by-day.

**Weekly ritual:** Sunday morning. Coffee. Open MFP web. Click each weekday's diary. Tab to Reports. Screenshot the macros chart. Paste into a training-log doc. Decide whether next week's calorie target moves up or down.

**Frustration:** Per-food granularity is gated behind a tier that still doesn't deliver it. The data is *hers*, MFP has it, and she cannot get a per-entry CSV without scraping.

### Persona 2: Devin, the AI nutrition-coach builder

**Today (without this CLI):** Devin read a Medium post called "How I Built a Nutrition Coach with Claude Code" and tried to replicate it. He spent two evenings gluing python-myfitnesspal into a script, hand-rolling cookie auth from `browser_cookie3`, then realized Claude Code couldn't reach his script reliably. He ended up copy-pasting yesterday's diary into Claude's chat window every morning so it can suggest the next meal. Sometimes Claude gets confused about which day it's looking at. He has no working MCP server; AdamWalt's repo is 11★ and he hasn't trusted it.

**Weekly ritual:** Every morning, paste yesterday's totals + today's planned meals into Claude. Ask for a recommendation that hits his macro targets. Re-type the recommended food into MFP by hand because the chat workflow can't write back.

**Frustration:** No first-class agent surface. The data round-trip is manual and error-prone, and the agent has no memory of the last 30 days unless he pastes it in every time.

### Persona 3: Priya, the weight-loss data analyst

**Today (without this CLI):** Priya is 14 months into a cut, down 38 lb. She is the type of person who built a personal Google Sheet tracking weight, body-fat %, neck/waist/hips, and weekly average calories. She manually transcribes her morning weigh-in from MFP into the sheet. She runs `=AVERAGE` formulas to smooth weight noise. When she wants to know "what was my weekly weight slope from week 12 to week 20 vs my deficit?" she copies columns into a second tab and runs a regression by hand. She has tried `seeM/myfitnesspal-to-sqlite` once; it errored on the captcha and she hasn't been back.

**Weekly ritual:** Monday weigh-in. Transcribe into sheet. Refresh the rolling 7-day average. Compare to last week's calorie deficit. Adjust deficit if the slope deviates from plan.

**Frustration:** Every datapoint MFP already has lives behind a manual transcription step. The reports endpoint *is* a JSON time-series — but you have to know the URL, and even then it's one nutrient at a time.

### Persona 4: Sam, the multi-app data archivist

**Today (without this CLI):** Sam syncs Apple Health, Garmin, Oura, and MFP. He has a Hugo-built personal site that publishes weekly self-quantified summaries. To get MFP data into his pipeline he uses `hbmartin/myfitnesspal-to-google-sheets` (14★, inactive, breaks every few months) and a cron'd Apps Script he doesn't fully understand. When the Google Sheet breaks, his weekly summary stalls.

**Weekly ritual:** Sunday night cron pulls MFP -> Sheets -> his static-site generator. Manually sanity-checks the diff before publishing.

**Frustration:** The MFP leg is the flakiest part of his stack and the only one without a maintained CLI.

## Candidates (pre-cut)

(See subagent transcript — 18 candidates generated, then cut to 12 survivors per the rubric.)

## Survivors

(See `2026-05-08-133559-feat-myfitnesspal-pp-cli-absorb-manifest.md` § Transcendence for the final table.)

## Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| C6 Recipe scaling | Monthly at best for the named personas; per-ingredient API loop adds verifiability cost without weekly leverage | C7 saved-meal expansion (covers the per-component nutrient pattern with the same shape) |
| C9 Meal-pattern detection | Generalizes into C2 (top-foods) and C13 (streak) plus the framework `sql` command; doesn't earn a standalone command slot | C2 top foods + C13 streak |
| C14 AI meal suggestion | Fails the LLM-dependency kill check (requires NLP ranking of foods to a goal) | C15 macro-gap candidate foods (mechanical reframe; pipe to `\| claude` for the language layer) |
| C16 Daily SQL pass-through | Already a generator-emitted built-in — `sql` is in `cobratree/classify.go.tmpl`'s `frameworkCommands`; counting it as novel double-counts | The framework `sql` command itself |
| C17 Web dashboard | Scope creep (server + TUI); rubric explicitly cuts dashboards | C8 agent context dump + C1 per-food CSV deliver the analytical value via one-shot commands |
| C18 Apple Health / Garmin import | External service not in the MFP spec; format coupling belongs in a separate cross-app CLI | C7 sync diff (covers the auditability frustration without crossing the spec boundary) |
