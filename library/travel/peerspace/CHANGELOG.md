# Changelog

## 2026.8.1 - 2026-08-17

- Baseline release metadata added for this published CLI.

## Unreleased (local amend, 2026-07-16)

Guest planning surface beyond search/shortlist — listing detail hydrate, calendar, host inquiry, verification.

### Added

- **`shortlist hydrate`** — `GET /v1/listings/{id}` for board or listing IDs; stores full page blocks in SQLite (about, rules, parking, cleaning, amenities, `format_fit`)
- **`venues get`** — live detail fetch + write-through (was local-only)
- **`calendar availability-start` / `availability-end` / `availability-month`**
- **`contracts guest-quote`** — request-to-book quote
- **`contracts inquiry-quote` / `inquiry-send`** — message-host flow (`inquiry-send` requires `--yes`)
- **`verification listing`** — guest verification check
- **`messages unread`** — v2 inbox unread count
- **`spaces faqs-event`** — space event FAQs
- **`shortlist create-board` / `shortlist add`** — favorite board writes (`project` board-id string body; `PSAccess` bearer)
- Export markdown includes About / Included / Host rules / Parking when hydrated

### Docs

- README, SKILL, AGENTS, root `--help` highlights, `which` index, tools-manifest, this changelog
- README recipes cover hydrate → calendar → inquiry-send → export; troubleshooting for empty export, board writes, rate limits

### Notes

- Search stays light; hydrate is opt-in after shortlisting
- Host contact is intentional only (`--yes` on `inquiry-send`); rate limits apply when contacting many hosts

---

This file is also maintained by printing-press-library release automation for published releases.
