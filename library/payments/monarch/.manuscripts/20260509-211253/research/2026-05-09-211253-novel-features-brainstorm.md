# Monarch Money — Novel Features Brainstorm (subagent output)

This file is the audit trail of the Phase 1.5c.5 brainstorm. The Customer model and Killed candidates do not flow into the absorb manifest, but they're persisted here for retro/dogfood debugging.

## Customer model

**Persona 1 — Priya, the dual-income budget couple's "CFO"**
- **Today (without this CLI):** Every Sunday Priya logs into app.monarch.com on her laptop, then opens four tabs: Transactions (filter "uncategorized, last 7 days"), Budget (current month), Cashflow (this month vs last), and a Google Sheet she copy-pastes net-worth and category-spend numbers into for her spouse. She can't answer "which categories have I quietly blown through 80% of by day 20?" without scrolling each budget row, and she can't see month-over-month merchant drift without exporting CSV.
- **Weekly ritual:** Categorize the week's transactions, reconcile any misfires, glance at budgets, push a one-paragraph "state of the household" to her spouse on Sunday night.
- **Frustration:** Bulk-categorizing 30-80 transactions a week through a web UI that wants one click per row, with no way to say "every Whole Foods purchase under $40 → Groceries → Weekly" as a rule and backfill the last 90 days at once.

**Persona 2 — Marcus, the FIRE-track engineer tracking 14 accounts**
- **Today (without this CLI):** Marcus has accounts at Fidelity, Schwab, Vanguard, two banks, a credit union, four credit cards, a HELOC, and Zillow-linked real estate. Each Saturday morning he refreshes the Monarch dashboard, screenshots the net-worth chart, and pastes new totals into his ten-year FIRE projection spreadsheet. He cannot tell from the UI which accounts are stale (last-synced > 24h), which institutions have silently broken, or whether the $4,200 jump in net worth this week came from market gains or a paycheck deposit.
- **Weekly ritual:** Refresh accounts, snapshot net worth, decompose the week-over-week delta into "income vs market vs spending," update FIRE date.
- **Frustration:** Monarch's net-worth chart is a single line; he wants the delta attribution and a stale-accounts alert before he trusts the number.

**Persona 3 — Devon, the freelancer juggling irregular income and recurring bills**
- **Today (without this CLI):** Devon's income is 6-12 client deposits a month at irregular cadence; his outflow is 22 recurring subscriptions and bills. He keeps a paper list of "expected paychecks this month" and crosses them off as they hit. The Monarch recurring panel shows next-due-date but doesn't surface "this Netflix charge went from $15.49 to $22.99 last cycle" or "this stream has missed its window — is it actually canceled?"
- **Weekly ritual:** Check what's hit, what's expected, whether any subscription quietly raised its price, project the next 30-day cash position.
- **Frustration:** Silent price drift on subscriptions (he found a $9 → $19 Adobe bump three months late) and not knowing on Wednesday whether Friday's rent will clear.

**Persona 4 — Sasha, the agent-power-user who wants Claude to "do the books"**
- **Today (without this CLI):** Sasha wants to ask Claude "what's my discretionary spend trend this quarter, exclude the medical category, and draft a paragraph for my therapist about it" in one conversation. Today she copy-pastes CSV exports from Monarch into Claude's context, which loses category structure, splits, and tags, and stays stale the moment she imports.
- **Weekly ritual:** Have an LLM read her current financial state and produce narrative output (memos to spouse, questions for advisor, journaling prompts).
- **Frustration:** No agent-native handle on her own data; the Monarch app is read-only-with-eyes, the official MCP doesn't exist, and TS MCP wrappers need Node + a session juggle.

## Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|------------|---------------------------|
| Transaction velocity (`transactions velocity`) | Glanceable chart, not a weekly decision driver; rolling-count anomaly flag has no clear action attached. | Cashflow forecast (#6) — same time-series source, actionable output. |
| Merchant intelligence (`merchants top`) | Already covered by absorbed `top-merchants` insight; reformatting an existing API surface is wrapper work, not transcendence. | Category leak detection (#7) — same merchant data, novel output. |
| Holdings drift vs target allocation (`holdings drift`) | Requires user to supply a target-allocation spec the brief never mentions; without it the feature is empty, with it it's a config-file feature, not a Monarch-native one. | Net-worth delta attribution (#2) — same Marcus persona, no user-supplied config needed. |
| Income-stream health (`recurring income-health`) | Subsumed by subscription price-drift detector applied to income-tagged streams; carving income off as a separate command duplicates the implementation without adding a distinct workflow. | Subscription price-drift detector (#3) — covers the same cadence/amount anomaly logic across all stream types. |
| Rule provenance audit (`rules audit`) | Useful but quarterly cadence at best (rules are set-and-forget); fails the weekly-use bar. | Bulk-categorize with rule generation (#1) — same rule entity, weekly cadence, action-shaped. |
