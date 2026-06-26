# Sigma Computing — Novel Features Brainstorm (audit trail)

## Customer model

**Priya — Sigma platform admin (mid-market SaaS, ~1,800 members, ~6,000 workbooks)**
- Today: Manages members/teams/grants entirely through the Sigma web UI plus a hand-rolled Python script that re-implements OAuth + cursor pagination every time it breaks. SCIM is gated behind Enterprise+SAML, which her org doesn't have, so the REST API is her *only* programmatic provisioning path.
- Weekly ritual: Monday onboarding/offboarding batch — create N new members, assign teams, set user attributes, deactivate leavers. Quarterly: a grant-audit fire drill when security asks "who can see the finance connection?"
- Frustration: No way to answer "who owns what" or "who can see what" without clicking through hundreds of workbooks. Offboarding a person leaves their workbooks orphaned and she only finds out when someone complains.

**Diego — analytics/data engineer (owns the BI delivery pipeline)**
- Today: Exports workbooks to CSV/PDF on a cron via curl scripts; manually re-copies template workbooks for new client tenants. Materialization schedules are managed by clicking into each workbook.
- Weekly ritual: Spin up workbook copies for new projects; check that overnight materializations ran; pull scheduled CSV/PDF exports for stakeholders.
- Frustration: `workbook copy` via the API silently assigns ownership to *him* (the calling admin), not the recipient — he discovers weeks later that 30 client workbooks are all owned by his service account. Materialization "run now" fails cryptically until the schedule has run at least once.

**Sam — BI governance / FinOps lead (part-time on Sigma, reports to CISO)**
- Today: Once a quarter, asks Priya to export member lists and grant CSVs, then pivots them in a spreadsheet to find stale content and over-broad access. Lives in Sigma's audit screens, which don't aggregate across resources.
- Weekly ritual: Reviews newly-shared workbooks; spot-checks that deactivated members lost access.
- Frustration: There is no single view of "stale workbooks nobody opened in 90 days," "members with grants but no team," or "connections shared org-wide." Every audit is a manual join across four UI surfaces.

## Candidates (pre-cut)

1. workbook copy w/ ownership repair (b)(f) — keep
2. grant audit <resource> (c) — keep
3. workbook stale (c) — keep
4. member offboard <email> (a)(b) — keep
5. ownership map (c) — KILLED (subsumed by offboard + stale)
6. member provision --from csv (a)(b) — keep
7. access review <member-email> (c) — keep
8. materialize run w/ schedule guard (b) — KILLED (wrapper + one guard; fold into absorbed command)
9. export bulk --query FTS (b)(c) — keep
10. team sync --from csv (a) — KILLED (overlaps member provision)
11. connection grants audit (c) — KILLED (subset of grant audit)
12. whoami/auth status (b) — KILLED (table-stakes, absorbed)
13. workbook lineage rollup (b)(c) — KILLED (weak transcendence proof)
14. tag report (c) — KILLED (thin group-by)
15. members orphaned-grants (c) — KILLED (folds into access review)
16. workbook diff-version (b) — KILLED (semantic diff = high build cost / LLM-ish)

## Survivors (7, all >= 6/10)

| Feature | Command | Score |
|---------|---------|-------|
| Grant audit (effective access) | `grant audit <resource>` | 8/10 |
| Workbook copy + ownership repair | `workbook copy <wb> --to <member>` | 8/10 |
| Stale-workbook detection | `workbook stale --days N` | 7/10 |
| Member offboard + content reassign | `member offboard <email> --transfer-to <member>` | 7/10 |
| Bulk member provisioning | `member provision --from <csv>` | 7/10 |
| Member access review (reverse audit) | `access review <member-email>` | 6/10 |
| Bulk export by search | `export bulk --query <FTS>` | 6/10 |

Killed: ownership map, materialize-run-guard, team sync, connection grants audit, whoami, lineage rollup, tag report, orphaned-grants, diff-version. (See reasons above.)
