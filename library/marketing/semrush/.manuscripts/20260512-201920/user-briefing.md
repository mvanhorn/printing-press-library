# User Briefing Context

## Vision
- Primary focus: SEO data tools — Domain Overview, Position Tracking (already use position tracking), Keyword Magic
- Wants to use SEMrush web interface (not API) to avoid Business-tier upgrade cost
- Currently on Guru tier
- Has 50,000 API credits available as fallback only

## Frustrations
- API requires Business+ tier (Guru users can't natively call API at scale)
- Output is currently CSV-only from UI; user wants structured Google Sheets export
- Most keyword research docs become noise — too many irrelevant keywords

## Killer Feature ("magic-recipe" keyword research)
1. Seed from high-conversion pages (user provides — exports from Google Analytics or pastes URLs)
2. Run seeds through Keyword Magic Tool to get variants
3. Look across Keyword Magic tabs: related terms, synonyms (often missed)
4. Score by Personalized Keyword Difficulty (PKD) — input client's domain to get PKD%
5. Filter on sweet spot: KD + volume + relevance
6. Output filtered keyword set into Google Sheets template (their format, client-ready)

## Auth Context
- On Guru tier in browser
- Logged into semrush.com in Chrome right now → AUTH_SESSION_AVAILABLE=true
- Will paste SEMrush API key as fallback (50k credits, for Phase 5 live smoke testing)

## Architectural Implications
- BROWSER_SNIFF_TARGET_URL=https://www.semrush.com (interface-first)
- Primary transport: browser-clearance HTTP (Surf + Chrome cookie import)
- API mode = explicit fallback, gated by --api flag or auto when interface call fails
- Google Sheets export = transcendence feature (likely needs OAuth or Sheets API service account)
