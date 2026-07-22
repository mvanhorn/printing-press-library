# Local Verification

Run date: 2026-06-21 UTC

## Live Source Checks

- `curl -I -L --max-time 20 https://www.paulgraham.com/articles.html`: HTTP/2 200, `content-type: text/html`.
- `curl -L --max-time 20 https://www.paulgraham.com/rss.html`: downloaded 6097 bytes.

## CLI Smoke Checks

```bash
go run ./cmd/paul-graham-pp-cli latest --json --limit 5
```

Returned five current index entries:

- How to Earn a Billion Dollars
- How to Convert Between Wealth and Income Tax
- The Brand Age
- The Shape of the Essay Field
- Good Writing

```bash
go run ./cmd/paul-graham-pp-cli search startup --json --limit 3
```

Returned matching essays including `pgh`, `before`, and `invtrend`.

```bash
go run ./cmd/paul-graham-pp-cli read greatwork --json --max-chars 500
```

Returned `How to Do Great Work`, slug `greatwork`, URL `https://www.paulgraham.com/greatwork.html`, word count `11941`, and truncated readable text.

```bash
go run ./cmd/paul-graham-pp-cli links greatwork --json
```

Returned extracted links from the essay page, including `https://www.paulgraham.com/index.html` and footnote anchors.
