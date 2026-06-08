# Release Readiness

This repo has three distinct release states. Treat them separately.

## 1. Local Source Checkout Works

Use this when you want to confirm the repo builds and tests as a source checkout.

```bash
make verify-release
```

Expected outcome:

- `go test ./...` passes
- `go vet ./...` passes
- both CLI binaries build via `make build-all`

## 2. Public Repo Ready

Use this before making the GitHub repository public.

Checklist:

- README explains current publication status and source-build path
- local-only artifacts are ignored by `.gitignore`
- CI workflows exist in `.github/workflows/`
- `make verify-release` passes
- no accidental session cookies, HAR files, or `.env` files are staged

Suggested checks:

```bash
git status --short
make verify-release
```

## 3. Printing Press Publish Ready

Use this before opening a library publish PR.

Install the validator if needed:

```bash
go install github.com/mvanhorn/cli-printing-press/v4/cmd/cli-printing-press@v4.20.1
```

Run the publish validator:

```bash
make validate-publish
```

Expected outcome:

- manifest metadata passes
- transcendence passes
- phase5 passes
- `govulncheck`, `go vet`, `go build`, `--help`, and `--version` pass

## Notes

- This document does not mean the repo has already been made public.
- This document does not mean a Printing Press library PR has already been opened.
- The local `os-ready` helper is useful as a smoke audit, but it does not understand Go repos fully and currently flags the lack of `pyproject.toml` or `package.json` as a false negative.
