# Phase 4.95 local code review: serply-pp-cli

- Review path chosen: direct inline review by the running agent of the hand-written files (internal/cli/serp_common.go, rank.go, serp_diff.go, research.go, serp_novel_test.go) plus `go vet ./...` and `go test -race` on the novel tests and a race-built live `research` run. Subagent dispatch was not available in this session (operator rule: no subagents), so the persona reviewers ran inline; /simplify was not run for the same reason.
- Autofix summary: 3 findings autofixed in-place across 2 rounds (device value case-sensitivity in validation, header and snapshot key; snapshot temp-file name collision under concurrent `serp diff` runs, now os.CreateTemp; plus the Phase 4.85 parser fixes). No commits: the working dir is not a git repo; the files are the record.
- Concurrency check: `research` shares one generated client across goroutines; the client's limiter and platform limiter maps are mutex-guarded and the race detector was clean on a live run.
- Timeout boundary: all three novel commands use `flags.newClient()` and `boundCtx`; no sibling internal package clients.
- Template-shape retro candidates:
  - internal/cli/which.go is not gofmt-clean as generated (template: which.go emit). Filed, not patched.
  - Novel scaffold help gate `hasChangedLocalFlags` ignores persistent flags, so `<novel> --dry-run` alone printed help (template: novel scaffold). Patched in the three hand-written commands, since those files are hand-authored now.
  - Root Short carries an em dash ("Serply CLI — ...") from the root template.
- Out-of-scope retro candidates: none.
- Surface-to-user findings: none.
- Convergence outcome: findings cleared at round 2.
