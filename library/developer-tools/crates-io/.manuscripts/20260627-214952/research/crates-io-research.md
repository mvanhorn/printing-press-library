# crates.io Registry API Research

## Source

- Official registry web API reference: `https://doc.rust-lang.org/cargo/reference/registry-web-api.html`
- Public registry host: `https://crates.io`
- Data access policy page checked: `https://crates.io/data-access`

## Scope

This print intentionally ships read-only ecosystem intelligence commands:

- Search crates with bounded `page` and `per_page` parameters.
- Inspect crate metadata, versions, owners, reverse dependencies, dependencies, and recent downloads.
- Expose local sync/search scaffolding for offline exploration without remote writes.

The mutating registry actions documented by Cargo, such as publish, yank, unyank, and owner changes, are not part of this CLI.

## Auth And Safety

The selected endpoints do not require a crates.io token. The generated client sends `Accept: application/json` and a CLI User-Agent when callers do not override headers. Live validation used the public API only.

## Validation Summary

`cli-printing-press dogfood --live --level full` passed with 46 exercised checks and 0 failures. The acceptance marker is archived in `../proofs/phase5-acceptance.json`.
