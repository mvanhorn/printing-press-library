# Local Validation Proofs

Run ID: `manual-20260727`

## Automated Tests

```bash
go test ./...
```

Current result:

```text
6 passed in 2 packages
```

Covered behavior:

- HTML search result parsing returns title, URL, and image URL.
- WordPress API fallback fetches featured image URL from the post endpoint.
- Empty query returns an empty slice.
- Download writes selected image and updates `download_path`.
- Out-of-range download index is rejected.
- Terminal preview renders Braille output from an image URL.

## Build

```bash
go build ./cmd/logodownload-pp-cli
```

Current result:

```text
success
```

## Live Smoke Tests

Search:

```bash
go run ./cmd/logodownload-pp-cli nike
```

Preview:

```bash
go run ./cmd/logodownload-pp-cli "Bradesco Logo" --preview --preview-h 6 --preview-w 20 --preview-limit 2
```

Download:

```bash
go run ./cmd/logodownload-pp-cli "Banco Inter" --download first --output-dir /tmp/logodownload-final-check
```

## Remaining Publication Caveat

These are local manual proofs. The canonical Printing Press publish flow should still generate official manuscripts, dogfood results, release metadata, registry updates, and skill mirrors.
