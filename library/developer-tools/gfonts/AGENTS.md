# gfonts — Agent Operating Guide

## Overview
Google Fonts CLI — zero-auth search, browse, and download of 1,900+ fonts.
Uses the public metadata endpoint that powers fonts.google.com.

## Binary
`gfonts-pp-cli` — single binary, no dependencies.

## Commands
- `search <query>` — search by name, category, or designer
- `list` — browse with --category, --sort, --limit
- `info <font>` — detailed metadata
- `download <font>` — download files (--variant, --output, --show)
- `trending` — trending fonts
- `categories` — category counts
- `random` — random font (--category)
- `agent-context` — emit the machine-readable command tree (JSON, --pretty for humans)

Every command supports `gfonts <cmd> --help` with usage and examples.

## Auth
None. All endpoints are public.

## Cache
Metadata cached at /tmp/gfonts-metadata-cache.json for 24 hours.
