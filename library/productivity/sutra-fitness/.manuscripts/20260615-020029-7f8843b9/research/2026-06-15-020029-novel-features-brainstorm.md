# Sutra Fitness — Novel Features Brainstorm (audit trail)

> Reframe note (applied in manifest): the subagent expressed several features as
> `analytics --type <X>`, but the generator emits a generic framework `analytics`
> command (`--type <resource> --group-by <field>`). To avoid collision, all
> transcendence analytics ship as DEDICATED top-level hand-written Cobra commands
> (`scorecard`, `no-shows`, `utilization`, `expiring`, `churn`, `revenue`,
> `referral-funnel`, `ltv`). Logic and personas below are preserved verbatim.

## Customer model

**Maya — Studio Owner / Operator (single or 2-3 location boutique).** Runs a yoga/pilates/cycle studio on Arketa. *Today:* lives in the Arketa dashboard, exports canned CSV/PDF reports she can't customize, and cannot get a clean answer to "which memberships expire this week" or "what's my no-show rate by instructor" without manual spreadsheet stitching. *Weekly ritual:* Monday revenue review, pulls a client list for an email blast, eyeballs next week's schedule for under-filled classes, decides which instructors to coach. *Frustration:* the vendor is "cagey" about client export, reporting is canned-only, and every metric she actually manages on is buried or absent.

**Dev — Front-Desk Lead.** Works the door during class transitions. *Today:* uses the vendor check-in screen on a tablet, hunts for booked names, manually books walk-ins, processes late cancels. *Weekly ritual:* opens the roster before each class, checks people in, books drop-ins, flags no-shows. *Frustration:* roster + spot map slow to load, walk-in booking for a brand-new client takes too many taps, no fast "who hasn't shown" view.

**Priya — Growth / Retention Manager (often the owner's second hat).** Owns membership renewals and referral campaigns. *Today:* manually scans purchases to find expiring packs and at-risk clients; has no view of referral conversion. *Weekly ritual:* builds a "reach out" list of expiring/low-credit clients, follows up with first-time visitors, checks who referred whom. *Frustration:* churn and at-risk signals live only in raw purchase rows; the referral funnel and first-visit follow-up are invisible without manual joins.

## Survivors (8, all hand-code, score >= 6/10)

| # | Feature | Command (reframed) | Score | Persona | How It Works |
|---|---------|--------------------|-------|---------|--------------|
| 1 | Instructor scorecard | `scorecard` | 9 | Maya | Join classes × reservations on class id → fill, no-show, check-in rates per instructor_name |
| 2 | No-show rate | `no-shows --group-by instructor\|class\|client` | 8 | Maya/Dev | Aggregate reservations.status=NO_SHOW over BOOKED, grouped |
| 3 | Capacity utilization | `utilization --group-by class\|instructor\|timeslot\|location` | 9 | Maya | total_booked/max_capacity per grouping over a date window |
| 4 | Expiring / low-balance | `expiring --within 7d --low-credits` | 9 | Priya | purchases.end_date within window OR remaining_uses low AND status=ACTIVE |
| 5 | Churn / at-risk | `churn --inactive-days 30` | 7 | Priya | clients × reservations × purchases: non-removed, no recent CHECKED_IN and/or EXPIRED plan (mechanical recency rule) |
| 6 | Revenue + prior-period | `revenue --group-by type\|location --compare-prior` | 8 | Maya | Sum purchases.price by type/location for window + delta vs prior equal window |
| 7 | Referral funnel | `referral-funnel` | 7 | Priya | referrals → client created → first purchase/check-in conversion + top referrers |
| 8 | Client LTV | `ltv --group-by client\|location` | 6 | Maya/Priya | Sum purchases.price per client + tenure from created_at, ranked |

## Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| Mark no-shows (`mark-no-shows`) | Unbuildable: API write surface is create/cancel/check-in only — no "set NO_SHOW" transition | `no-shows` (reads existing NO_SHOW status) |
| Schedule gap / underfilled finder | Sibling overlap: a `--threshold` filter on the utilization join | `utilization` |
| Client segment export | Export is framework-absorbed; `--segment` duplicates expiring/churn predicates | `expiring` + `churn` |
| Membership status breakdown | Thin GROUP BY status; covered by neighbors | `expiring` / `revenue` |
| Retention cohort matrix | Verifiability + build cost; churn covers actionable retention | `churn` |
| Roster w/ spot map | Cut to hold survivor target (high weekly-use; candidate to re-add at gate) | reservations mirror + `utilization` |
| First-visit follow-up | Narrow; overlaps churn/onboarding (candidate to re-add at gate) | `churn` |
| Daily door brief | Single-day slice of utilization; scope-trimmed (candidate to re-add at gate) | `utilization` |
