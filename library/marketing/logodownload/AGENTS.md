# LogoDownload CLI Notes

This directory contains `logodownload-pp-cli`, a read-only-by-default Printing Press style CLI for finding public logo entries on logodownload.org.

## Local Validation

Run these before handing off changes:

```bash
go mod tidy
go test ./...
go vet ./...
go build ./cmd/logodownload-pp-cli
govulncheck ./...
go run ./cmd/logodownload-pp-cli --help
go run ./cmd/logodownload-pp-cli --version
go run ./cmd/logodownload-pp-cli search nike
go run ./cmd/logodownload-pp-cli search "Bradesco Logo" --preview --preview-limit 2
```

For download validation, use a temporary output directory:

```bash
go run ./cmd/logodownload-pp-cli search "Banco Inter" --download first --output-dir /tmp/logodownload-check
```

## Behavior Contract

- JSON results must be printed to stdout.
- Logs, warnings, and terminal previews must be printed to stderr.
- Empty searches must return `[]`, not `null`.
- Search and preview must not write logo files.
- `--download` is the only command path that writes logo image files.
- Do not add authentication or scraping bypass behavior.

## Publishing Note

This local project is not yet in the canonical `mvanhorn/printing-press-library` shape. New CLI publication should be routed through `cli-printing-press` and `/printing-press-publish`, which generates manuscripts, registry metadata, skill mirrors, release files, and the final PR shape.
