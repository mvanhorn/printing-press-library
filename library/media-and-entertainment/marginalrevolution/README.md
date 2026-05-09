# Marginal Revolution CLI

`marginalrevolution-pp-cli` reads Marginal Revolution through its public RSS feed. It is read-only, unauthenticated, and useful for agent workflows that need recent posts, authors, categories, comment counts, and outbound links without browser automation.

## Install

```bash
npx -y @mvanhorn/printing-press install marginalrevolution --cli-only
```

Direct Go install:

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/marginalrevolution/cmd/marginalrevolution-pp-cli@latest
```

## Commands

```bash
marginalrevolution-pp-cli latest --limit 5
marginalrevolution-pp-cli latest --author Tyler --json
marginalrevolution-pp-cli search "ai" --category Web/Tech --agent
marginalrevolution-pp-cli read "self-fulfilling-misalignment"
marginalrevolution-pp-cli links --limit 3
marginalrevolution-pp-cli categories
marginalrevolution-pp-cli authors
marginalrevolution-pp-cli doctor --json
```

## Notes

Marginal Revolution's public RSS feed is available at `https://marginalrevolution.com/feed`. The normal WordPress JSON and site-search endpoints can return Cloudflare browser challenges to command-line clients, so this CLI intentionally keeps search scoped to the current feed.
