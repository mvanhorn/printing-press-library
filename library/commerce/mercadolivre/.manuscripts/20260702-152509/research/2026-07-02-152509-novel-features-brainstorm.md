# Novel Features Brainstorm — mercadolivre-pp-cli (audit trail)

Subagent output preserved for retro/dogfood. 6 survivors (all >=7/10). See absorb manifest for the shipping transcendence table.

## Customer model
- Renata — suprimentos analyst: 6 tabs + manual spreadsheet + spec-table screenshots; "220V + >=700W cheapest" is manual eyeballing; Monday's quote stale by PO clearance.
- suprimentos_ia — dept automation agent: no scriptable path today (official API 403 + captcha wall); needs agent-shaped JSON/CSV + normalized spec columns.
- Márcio — procurement approver: needs price-at-decision audit trail + drift alerts + freshness signal.

## Survivors (shipping scope)
1. compare <ids> [--diff] — aligned spec matrix (10/10)
2. cheapest --query --spec ... — cheapest meeting spec floor (9/10)
3. dispersion MLB<id> — cross-seller price spread (9/10)
4. price-history <product> [--since] — price drift (9/10)
5. cotacao <ids> [--format] — quotation bundle with provenance (8/10)
6. stale [--older-than] — freshness gate (7/10)

## Killed
spec-diff (folded into compare --diff), value-rank (invented/brittle), catalog-vs-listing (sibling of dispersion), brand-compare (weaker slice), seller-report (thin data), spec-coverage (fold into compare), watch/re-quote (= price-history --since), spec-filter (= cheapest minus sort).
