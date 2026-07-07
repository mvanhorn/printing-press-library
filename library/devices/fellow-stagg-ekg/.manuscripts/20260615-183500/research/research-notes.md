# Fellow Stagg EKG research notes

- Source of truth for command behavior: the working Python CLI in `apps/fellow-stagg-ekg-cli/`.
- Public library package mirrors the same kettle HTTP command set.
- Base transport is `GET /cli?cmd=...`.
- Key commands included in the published CLI: `status`, `state`, `settings`, `clock`, `info`, `heat`, `off`, `set-temp`, `set-setting`, `units`, `button`, `dial`, `beep`, and `raw`.

