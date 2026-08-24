# Changelog

This file is maintained by printing-press-library release automation. Do not hand-edit release sections in normal PRs.

## Unreleased

### Fixed

- Strip the DSM `{"success":true,"data":{...}}` transport envelope centrally in the client, so commands, the local cache and the MCP server see the payload instead of the wrapper. Every command previously printed an object, which also meant the table renderer - it only fires for a JSON array - never ran anywhere in the CLI.
- Unwrap DSM list payloads, which nest the rows one level deeper under a per-resource key (`{"offset":0,"total":14,"users":[...]}`). List commands with no explicit response path now return the row array, reusing the framework's existing single-array-sibling heuristic and gated on `isList` so a single-object GET carrying an array field is never collapsed.
- Redact `sid` and `synotoken` recursively. They live inside the DSM envelope, and the previous top-level-only sweep printed both in cleartext on `session login --agent`.
- Render DSM 4xx error codes as credential failures only for `SYNO.API.Auth` calls. Those numbers are namespace-specific, so `files list` reported "invalid account or password" for what is an invalid file-operation parameter.
- Correct the response paths and endpoints for `storage disks`, `storage pools`, `storage volumes` and `system services`, which called namespaces DSM 7.1.1 answers with error 101 or 103.
- Replace placeholder examples (`example-resource`, `example-value`) with values verified against a live DS415+ for `user get`, `group members`, `folder get`, `folder permissions`, `storage smart` and `files list`. `files stat --path` takes a JSON array of paths, not a bare path; its example said otherwise and failed to parse.

- Give the five mutating File Station commands (`files mkdir`, `files rename`, `files download`, `files copy-start`, `files delete-start`) examples whose `--path`, `--folder-path` and `--name` values are valid JSON arrays. The generated placeholders were bare strings and failed the client-side JSON check before any request was built.
- Read the SMART attribute table from the `smartInfo` response key and document that `--device` takes the device path (`/dev/sda`) that `storage disks` reports, not the bare disk id. The command previously printed an empty table and DSM answered error 117 for an id.
- Mark `files download` as a binary-response endpoint so the transport wraps the file in the base64 envelope instead of failing the JSON parse and reporting an expired session.
- Default `folder permissions --user-group-type` to `local_user` and read the rows from the `items` response key. DSM answers error 403 when the parameter is absent, so a bare `--name <share>` call could never succeed.

- Fold every live-verified correction back into `spec.yaml`, so a regeneration reproduces them instead of reverting to the placeholder output: the `load_info` endpoints behind `storage disks`, `storage pools` and `storage volumes`, the `SYNO.Core.Service` version-3 `get` call behind `system services`, all six response paths (`disks`, `storagePools`, `volumes`, `smartInfo`, `items`, `service`), the twelve corrected command examples, the `folder permissions --user-group-type` default, `files download` as a binary-response endpoint, and the removal of the account-to-groups endpoint.

### Changed

- Rewrite the root command description and the `learnings list` summary so they match the API narrative instead of the generator's placeholder wording.
- Replace nine generated MCP tool descriptions (`files_delete_stop`, `files_info`, `folder_list`, `group_list`, `session_logout`, `storage_esata`, `storage_luns`, `storage_smart_schedule`, `storage_usb`) with agent-grade text grounded in the endpoint semantics, clearing every pending tools-audit finding.

### Removed

- Drop the `user groups` command and the matching `user_groups` MCP tool. `SYNO.Core.User.Group` implements only `join` on DSM 7.1.1 and answers `list` with error 103, so there is no single-call account-to-groups lookup to expose. Use `group list` plus `group members` instead.

