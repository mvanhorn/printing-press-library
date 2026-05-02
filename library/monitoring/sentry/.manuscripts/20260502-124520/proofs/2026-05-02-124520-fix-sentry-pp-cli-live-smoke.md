# Sentry Live Smoke

Token source: SENTRY_AUTH_TOKEN (value redacted).

- doctor --json: PASS
- organizations list --json --select slug,name: PASS (1 organization(s); slug redacted)
- seer --json: PASS (13 model name(s))
- organizations projects list-an-organization-s <redacted-org> --json --select slug,name: PASS (5 project(s))
- organizations issues list-an-organization-s <redacted-org> --limit 3 --json --select shortId,title,count,userCount: PASS (3 issue(s))
- sync --resources organizations --max-pages 1 --json: PASS (0 resource completion event(s))

Acceptance Report: sentry
  Level: Quick live smoke (read-only)
  Tests: 6/6 passed
  Gate: PASS
