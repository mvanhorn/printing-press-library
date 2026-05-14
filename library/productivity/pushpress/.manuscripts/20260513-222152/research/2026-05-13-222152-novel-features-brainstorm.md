# Novel Features Brainstorm — pushpress-pp-cli

## Customer model

**Alex — i2 Fitness operator.** Monday-morning churn review ritual: who went dark last week, who's mid-cancellation, what plans are bleeding. Frustration: the "members who haven't checked in" view is buried in the dashboard; can't pipe; can't cross-reference with GHL.

**Trainer-dashboard build agent.** Needs each coach's roster with last-visit + plan + status; chokes on `/v3/customers` paginated page/limit with no server-side filters.

**Business-dashboard cron.** Nightly job needs counts (signups, check-ins, active members, going-dark) as deterministic JSON.

## Survivors (7, all ≥6/10)

| # | Feature | Command | Score | How It Works | Persona |
|---|---------|---------|-------|--------------|---------|
| 1 | Going-dark report | `going-dark --days N` | 9/10 | Local SQLite join of synced `customers` × `checkins`; rows where `MAX(checkins.timestamp) < now - N days` AND `status = active`, or never checked in | Alex; cron |
| 2 | Recency ladder | `recency [--bucket 7,14,30,60,90]` | 8/10 | `GROUP BY` over days-since-last-checkin from local store; emits count + sample names per bucket | Alex; cron |
| 3 | Trainer roster | `roster` | 7/10 | Local join `customers × MAX(checkin.timestamp)`; one line per row: id, name, plan, status, last_visit, days_since | Trainers; Alex |
| 4 | Daily KPI ticker | `kpi today` | 8/10 | One pass over local store: signups_today, checkins_today, active_members, going_dark_14d, going_dark_30d; JSON by default | cron; Alex |
| 5 | Member 360 | `member <id\|email>` | 7/10 | `/v3/customers/{id}` (or local lookup) + last 10 checkins + streak + cadence trend (last 4 weeks vs prior 4) | Trainers; Alex |
| 6 | Cohort retention | `cohort --month YYYY-MM` | 6/10 | For customers `dateAdded` ∈ month: % with any checkin in days 0-30 / 0-60 / 0-90 post-join | Alex (monthly) |
| 7 | Class-type mix | `class-mix [--days N]` | 6/10 | Histogram of `ClassCheckin.className` from local `checkins` over window; counts + % share | Alex (weekly) |

## Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---|---|---|
| Attendance streak per member (standalone) | Vanity metric for members not operators; folded into Member 360 | Member 360 |
| Check-in cadence per member (standalone) | Folded into Member 360 trend line; not a weekly command on its own | Member 360 |
| Birthday/anniversary list | `customers.birthday` field not confirmed in /v3 spec — verifiability fails | none |
| Webhook reachability check | No /v3 test-fire endpoint; scope creep | absorbed |
| Cross-CLI xref stub | Brief explicitly defers cross-join this run; thin rename of search | absorb #3 |
| Active member roll | Thin filter over roster | Roster |
| Dry-run message preview | Already absorbed | absorb #9-11 |
