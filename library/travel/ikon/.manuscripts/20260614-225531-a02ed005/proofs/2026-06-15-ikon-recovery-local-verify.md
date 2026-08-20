# Ikon recovery local verification

Date: 2026-06-15
Run: 20260614-225531-a02ed005
Scope: local recovery and promotion only; no publish, push, PR, or authenticated Ikon calls.

## Recovery actions

- Archived stale lock:
  `/Users/pbl-dev/printing-press/.locks/ikon-pp-cli.lock.stale-20260615-005619`
- Promoted interrupted working tree from:
  `/Users/pbl-dev/printing-press/.runstate/pbl-415c512c/runs/20260614-225531-a02ed005/working/ikon-pp-cli`
- Promoted local library destination:
  `/Users/pbl-dev/printing-press/library/ikon`
- Copied runstate evidence, excluding `working/`, into:
  `/Users/pbl-dev/printing-press/library/ikon/.manuscripts/20260614-225531-a02ed005`

## Verification from interrupted working tree

Passed:

```bash
go test ./...
go mod tidy
go vet ./...
go build ./...
./ikon-pp-cli --help
./ikon-pp-cli version
./ikon-pp-cli doctor
```

Observed:

- `./ikon-pp-cli version` reported `ikon-pp-cli 1.0.0`.
- `./ikon-pp-cli doctor` reported API reachable.
- `./ikon-pp-cli doctor` reported auth not configured, expected for no-auth recovery.

Not rerun:

- `govulncheck ./...` was present in the original generation proof, but `govulncheck` was not on PATH in this recovery session.

## Verification from promoted library tree

Passed from `/Users/pbl-dev/printing-press/library/ikon`:

```bash
go test ./...
go mod tidy
go vet ./...
go build ./...
./ikon-pp-cli --help
./ikon-pp-cli version
./ikon-pp-cli doctor
```

## Hand-built novel-command recovery

Claude stopped after creating `ikon_common.go`; `most-visited` was already
hand-authored, but these generated stubs still returned TODO errors:

- `plan`
- `calendar`
- `watch`
- `changes`

Recovered locally:

- Implemented `plan` as a reservation-window fanout across reservation-enabled
  resorts.
- Implemented `calendar` as a per-day month status view for one resort.
- Implemented `watch` as a resort/date poller, with agent/no-input mode bounded
  to one check unless `--max-checks` is explicitly provided.
- Implemented `changes` as local SQLite-backed availability snapshot diffing.
- Added the `availability_snapshots` local store table.
- Added focused helper tests in `internal/cli/ikon_common_test.go`.
- Rebuilt checked-in local binaries:
  `ikon-pp-cli` and `ikon-pp-mcp`.

Passed after hand-build recovery:

```bash
go test ./...
go mod tidy
go vet ./...
go build ./...
go build -o ikon-pp-cli ./cmd/ikon-pp-cli
go build -o ikon-pp-mcp ./cmd/ikon-pp-mcp
./ikon-pp-cli --help
./ikon-pp-cli version
./ikon-pp-cli doctor
./ikon-pp-cli plan --dry-run --from 2026-01-10 --to 2026-01-20
./ikon-pp-cli calendar --dry-run 14 --month 2026-01
./ikon-pp-cli watch --dry-run 14 2026-01-17
./ikon-pp-cli changes --dry-run
```

Observed:

- No `TODO: implement novel feature` stubs remain under `internal/cli`.

## Local packaging recovery

Completed:

- Added `pp:data-source` annotations to all five novel commands.
- Refreshed local root binaries:
  `ikon-pp-cli` and `ikon-pp-mcp`.
- Refreshed local stage binaries under `build/stage/bin/`.
- Repacked `build/ikon-pp-mcp-darwin-arm64.mcpb`.
- Verified MCPB zip integrity with `unzip -t`.
- Added package evidence:
  `dogfood-results.json` and `workflow-verify-report.json`.
- Prepared local publish-repo diff under:
  `/Users/pbl-dev/printing-press/.publish-repo-pbl-415c512c/library/travel/ikon`
- Mirrored focused skill to:
  `/Users/pbl-dev/printing-press/.publish-repo-pbl-415c512c/cli-skills/pp-ikon/SKILL.md`

Passed from the publish-repo copy:

```bash
go test ./...
go mod tidy
go vet ./...
go build ./...
go build -o /tmp/ikon-publish-check-cli ./cmd/ikon-pp-cli
go build -o /tmp/ikon-publish-check-mcp ./cmd/ikon-pp-mcp
/tmp/ikon-publish-check-cli --help
/tmp/ikon-publish-check-cli version
/tmp/ikon-publish-check-cli doctor
/tmp/ikon-publish-check-cli plan --dry-run --from 2026-01-10 --to 2026-01-20
/tmp/ikon-publish-check-cli calendar --dry-run 14 --month 2026-01
/tmp/ikon-publish-check-cli watch --dry-run 14 2026-01-17
/tmp/ikon-publish-check-cli changes --dry-run
git diff --check -- library/travel/ikon cli-skills/pp-ikon
go run ./tools/generate-registry/main.go --validate ikon
```

Notes:

- The publish-package verifier script compares `origin/main...HEAD`, so it
  does not inspect uncommitted new Ikon files without a local commit. No local
  commit was created in this recovery lane.
- Catalog regeneration would rewrite shared catalog files such as `README.md`;
  that checkout already had unrelated dirty changes, so catalog write is left
  for the explicit publish/PR lane.

## Canonical Printing Press install evaluation

Codex PATH did not include the real generator binary, but it was installed at:

```bash
/Users/pbl-dev/go/bin/cli-printing-press
```

The unrelated `/opt/homebrew/bin/pp` binary is macOS ASN.1 pretty-print tooling,
not Printing Press.

Canonical commands run:

```bash
/Users/pbl-dev/go/bin/cli-printing-press publish validate --dir /Users/pbl-dev/printing-press/library/ikon --json
/Users/pbl-dev/go/bin/cli-printing-press dogfood --dir /Users/pbl-dev/printing-press/library/ikon --spec /Users/pbl-dev/printing-press/library/ikon/spec.yaml --research-dir /Users/pbl-dev/printing-press/library/ikon/.manuscripts/20260614-225531-a02ed005 --json
/Users/pbl-dev/go/bin/cli-printing-press shipcheck --dir /Users/pbl-dev/printing-press/library/ikon --spec /Users/pbl-dev/printing-press/library/ikon/spec.yaml --research-dir /Users/pbl-dev/printing-press/library/ikon/.manuscripts/20260614-225531-a02ed005 --json
/Users/pbl-dev/go/bin/cli-printing-press bundle /Users/pbl-dev/printing-press/library/ikon --platform darwin/arm64 --version 1.0.0
/Users/pbl-dev/go/bin/cli-printing-press publish package --dir /Users/pbl-dev/printing-press/library/ikon --category travel --target /Users/pbl-dev/Documents/Codex/2026-06-15/printing-press-ikon-workroom/work/ikon-publish-staging --json
```

Results:

- `publish validate` initially caught missing `novel_features` in
  `.printing-press.json`; fixed from `research.json`.
- `publish validate` caught missing Phase 5 marker; a skip marker is not
  acceptable for Ikon because cookie/browser-session auth APIs require Phase 5
  acceptance.
- `dogfood` passed after removing dead helper `formatCLIParamValue`.
- After Chrome-session auth was approved, live Phase 5 passed and
  `publish validate` passed.
- `shipcheck` passed all six canonical Phase 4 legs.
- `bundle` passed and refreshed the MCPB.
- `publish package` passed to local staging. No publish, push, PR, or GitHub
  mutation was performed.

## Auth-assisted recovery continuation

Greg logged in to Ikon in Chrome and authorized using that local browser session
for verification.

Completed:

- Installed a local workroom `pycookiecheat` runtime so `auth login --chrome`
  could import cookies without changing system Python.
- Fixed Ikon cookie auth to import both top-level `ikonpass.com` and
  `account.ikonpass.com` cookies.
- Fixed cookie persistence so `.ikonpass.com` and account-domain cookies are
  available to direct CLI runs, staged binaries, MCP shellouts, and verifier
  environments that have config but not the persisted cookie jar.
- Updated runnable examples from unpublished `ikon ...` alias form to the
  shipped `ikon-pp-cli ...` binary name.
- Removed the rejected Phase 5 skip marker after live acceptance passed.
- Deleted temporary live-output files from `/tmp`.

Live auth evidence, without storing customer values:

```bash
PATH="/Users/pbl-dev/Documents/Codex/2026-06-15/printing-press-ikon-workroom/work/bin:$PATH" \
  ./ikon-pp-cli auth login --chrome --no-input --yes

./ikon-pp-cli doctor --agent
./ikon-pp-cli most-visited --agent --no-cache --select resort_name,total_days,seasons
build/stage/bin/ikon-pp-cli calendar 14 --month 2026-01 --agent --no-cache --select resort_id,month
build/stage/bin/ikon-pp-cli plan --from 2026-01-10 --to 2026-01-20 --agent --no-cache
```

Observed:

- `doctor` reported credentials valid.
- Direct root and staged live probes succeeded.
- Phase 5 acceptance is summary-only and stored at
  `proofs/phase5-acceptance.json`.

Phase 5 command:

```bash
/Users/pbl-dev/go/bin/cli-printing-press dogfood \
  --dir /Users/pbl-dev/printing-press/library/ikon \
  --spec /Users/pbl-dev/printing-press/library/ikon/spec.yaml \
  --research-dir /Users/pbl-dev/printing-press/library/ikon/.manuscripts/20260614-225531-a02ed005 \
  --live --level quick \
  --write-acceptance /Users/pbl-dev/printing-press/library/ikon/.manuscripts/20260614-225531-a02ed005/proofs/phase5-acceptance.json \
  --json
```

Phase 5 result:

- `status`: `pass`
- `matrix_size`: `14`
- `tests_passed`: `14`
- `tests_skipped`: `10`
- `auth_context.type`: `cookie`

Final local verification after auth and docs fixes:

```bash
go test ./...
go mod tidy
go vet ./...
go build ./...
go build -o ikon-pp-cli ./cmd/ikon-pp-cli
go build -o ikon-pp-mcp ./cmd/ikon-pp-mcp
/Users/pbl-dev/go/bin/cli-printing-press bundle /Users/pbl-dev/printing-press/library/ikon --platform darwin/arm64 --version 1.0.0
/Users/pbl-dev/go/bin/cli-printing-press verify-skill --dir /Users/pbl-dev/printing-press/library/ikon
/Users/pbl-dev/go/bin/cli-printing-press validate-narrative --strict --full-examples --research /Users/pbl-dev/printing-press/library/ikon/.manuscripts/20260614-225531-a02ed005/research.json --binary /Users/pbl-dev/printing-press/library/ikon/ikon-pp-cli
/Users/pbl-dev/go/bin/cli-printing-press shipcheck --dir /Users/pbl-dev/printing-press/library/ikon --spec /Users/pbl-dev/printing-press/library/ikon/spec.yaml --research-dir /Users/pbl-dev/printing-press/library/ikon/.manuscripts/20260614-225531-a02ed005 --json
/Users/pbl-dev/go/bin/cli-printing-press publish validate --dir /Users/pbl-dev/printing-press/library/ikon --json
/Users/pbl-dev/go/bin/cli-printing-press pii-audit /Users/pbl-dev/printing-press/library/ikon --manuscripts-dir /Users/pbl-dev/printing-press/library/ikon/.manuscripts/20260614-225531-a02ed005 --json
/Users/pbl-dev/go/bin/cli-printing-press tools-audit /Users/pbl-dev/printing-press/library/ikon --json
```

Final results:

- Go tests, vet, build, and explicit CLI/MCP binary builds passed.
- `bundle` passed and refreshed `build/ikon-pp-mcp-darwin-arm64.mcpb`.
- `verify-skill` passed.
- `validate-narrative` passed: 8 narrative commands resolved and full examples
  passed.
- `shipcheck` passed all six legs.
- `publish validate` passed.
- `pii-audit` returned `null`.
- `tools-audit` returned `null`.
- `publish package` passed to local staging:
  `/Users/pbl-dev/Documents/Codex/2026-06-15/printing-press-ikon-workroom/work/ikon-package-stage-20260615013632-final/library/travel/ikon`
- `mcp-audit` reports Ikon has MCP over both transports with 11 endpoint-mirror
  tools; recommendation remains to declare higher-level `mcp.intents`.

Scorecard note:

- `scorecard --live-check` exits 0 with grade `A` and total `80`.
- Its sampled live feature output still reports 401s for three features even
  though direct root and staged no-cache live probes pass. Treat that as a
  scorecard sampler mismatch, not an Ikon session blocker.

Publish diff:

- Prepared under:
  `/Users/pbl-dev/printing-press/.publish-repo-pbl-415c512c/library/travel/ikon`
- Focused skill mirrored to:
  `/Users/pbl-dev/printing-press/.publish-repo-pbl-415c512c/cli-skills/pp-ikon/SKILL.md`
- `git diff --check -- library/travel/ikon cli-skills/pp-ikon` passed.
- Binaries and MCPB build outputs are excluded from the publish diff.
- The publish checkout still has unrelated dirty files outside Ikon; those were
  not touched.

Installer-path check:

```bash
npx -y @mvanhorn/printing-press-library search ikon --json
npx -y @mvanhorn/printing-press-library install ikon --json
go run ./tools/generate-registry/main.go --validate ikon
```

Results:

- Public catalog search returned `[]`.
- Public installer returned `not in catalog` for `ikon`.
- Local publish-repo registry validation passed for the prepared Ikon entry:
  `Registry validation passed (1 entries).`
- Interpretation: the installer path cannot succeed until the prepared Ikon
  catalog/library diff is published. This is now a catalog/publish boundary,
  not a recovery/build/auth blocker.

Current true boundary:

- `needs publish decision`: local recovery, live Phase 5, canonical validation,
  PII/tools audits, local package staging, and publish diff preparation are done.
  No GitHub mutation, push, PR, or publish has been performed.
