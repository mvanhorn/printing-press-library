## Customer model

### Persona 1: Facility Owner-Operator (the brief's user — the facility admin)
**Today (without this CLI):** Logs into app.goose.pet every morning, clicks Dashboard, then Bookings, then exports CSVs one report at a time from the Reports page. Pulls customer balances by clicking into each customer one at a time. Cross-checks vaccinations by opening each pet profile.

**Weekly ritual:** Sunday-night planning — pulls the upcoming-week booking list (boarding + daycare), spot-checks vaccination expirations, reviews outstanding balances, exports the sales report for last week, and reconciles vouchers/credits. Monday-morning prep — runs Feeding & Medication report and prints the day's roster with pet tags + room assignments.

**Frustration:** The web app has ~50 report types behind embedded Explo dashboards he can read but not script against. There is no API documentation, no bulk customer query, no way to grep notes across customers, no way to pipe a vaccination-expiring list into a text-blast tool. Every batch action becomes a click-loop.

### Persona 2: Front-Desk Lead (morning-shift operator)
**Today (without this CLI):** Opens the Goose dashboard at 7am, alt-tabs between Dashboard (arrivals/departures), the grooming Scheduler, each pet's feeding instructions tab, and customer profiles for incoming pickups. Prints the Feeding & Medication report from the Reports page. Manually phones customers about expired vaccines on check-in day.

**Weekly ritual:** Daily morning roster build — who is checking in, who is checking out, who is here overnight, what room/yard each pet is in, who has medication at AM/PM, which incoming pets have lapsed vaccinations or unsigned agreements, which checkouts have unpaid balances.

**Frustration:** Information for the morning huddle is spread across five web-app screens. By the time she has assembled it, the first customers are walking in. She cannot pipe the day's roster into the print queue, into the staff Slack, or into a kitchen-board display.

### Persona 3: Revenue/Marketing Analyst (part-time, weekly cadence)
**Today (without this CLI):** Logs in weekly, clicks through ~16 CSV export buttons one at a time, downloads to local Excel, manually concatenates weeks for trend analysis. Opens Explo embed dashboards but cannot pivot or join across them. No way to ask "which customers haven't booked in 90 days but have an unused voucher."

**Weekly ritual:** Monday — exports sales report, customer-activity report, agreement report, voucher report. Builds churn list (last-booked > 60d) manually by sorting customers by last visit in the web app.

**Frustration:** Cross-entity questions (vouchers × bookings × customers × tags) are impossible in the web app. Every analytical question becomes a multi-CSV stitch in Excel.

### Persona 4: On-Call Operator / Off-Hours Agent (after-hours phone, working from home)
**Today (without this CLI):** Customer calls Saturday night asking about Monday's drop-off — agent must log into web app from home, search customer, click into the booking, scroll for feeding instructions, check vaccinations, read notes. Slow on mobile/laptop tether.

**Weekly ritual:** 3-5 after-hours calls per week — each requires looking up a customer + pet + upcoming booking + balance + notes.

**Frustration:** No fast way to answer "what's the deal with this customer" from a terminal. Web app is several clicks deep.

## Candidates (pre-cut)

(See subagent run — 18 candidates generated. Survivors listed below; kills tabulated in the manifest's `### Killed candidates` section.)

## Survivors and kills

See the absorb manifest (`2026-05-10-205901-feat-goose-pp-cli-absorb-manifest.md`) — `### Transcendence` table for survivors and `### Killed candidates` for the dropped set.
