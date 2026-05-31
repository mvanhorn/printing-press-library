# here.now CLI — Novel Features Brainstorm (subagent audit trail)

## Customer model

**Persona 1 — "Mira, the agent-builder shipping demos"** (free plan). Publishes 10–30 throwaway sites/week, most anonymous. Frustration: anonymous sites expire in 24h and she loses the ones she meant to keep because the claimToken is buried in terminal scrollback.

**Persona 2 — "Devin, the free-tier power user with one drive."** Treats his single free Drive as an agent's canonical store, syncs a local folder to it. Frustration: no diff — every sync is a full manual re-upload via scripts/drive.sh, burning rate budget.

**Persona 3 — "Sara, the form-collector running Site Data."** Static landing pages collecting form submissions into Site Data collections. Frustration: submissions siloed per-site/per-collection; no cross-site "what came in this week", no CSV export.

**Persona 4 — "Theo, the budget-conscious publisher near the free ceiling."** Runs near 500 sites / 10GB / 60-publish-hour. Frustration: no free way to see usage vs limits, what's stale, or what's expiring — analytics is paywalled, the only health signal is a hard failure.

## Survivors (8, all hand-code, all >= 7/10)

1. Claim-token vault + auto-claim (9/10) — Mira
2. Expiry radar (8/10) — Mira, Theo
3. Drive sync (sha256 diff push/pull) (9/10) — Devin
4. Drive diff (dry-run drift) (7/10) — Devin
5. Cross-site Site Data search (8/10) — Sara
6. Free-plan usage meter (8/10) — Theo + all free users
7. Stale-site finder (7/10) — Theo
8. Publish resume (finish half-done publish) (7/10) — Mira

## Killed candidates
- Site Data CSV export — leans on global --csv flag, not its own command (→ folded into site-data list --csv)
- Duplicate-site detector — depends on unsynced version content hash
- doctor plan-aware — doctor is framework-standard; plan-detection is a flag on it
- Password-gated bulk audit — thin metadata projection (sites list --select)
- Backup/export everything — scope creep, overlaps sync
- "What changed" feed — overlaps claims/stale/search
- Link/handle router map — handles/links are paid+single on free tier, low weekly use
- Anonymous→permanent migrate — composite where handle is paid; durable half is the vault auto-claim
