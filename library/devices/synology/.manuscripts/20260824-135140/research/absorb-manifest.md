# Absorb Manifest - synology-pp-cli

## Ecosystem scan

| Tool | Kind | Status | What it covers |
|------|------|--------|----------------|
| `synology-mcp` v0.1.0 (installed locally, read from source: 3444 lines across 8 service modules) | MCP server | Active, the user's stated capability floor | 39 SYNO namespaces: auth, DSM info, system, utilization, network, package, share, share permission, user, group, group member, storage disk/pool/volume, syslog, NFS, UPS, iSCSI LUN, SMART, File Station (list, search, download, copy/move, create folder, delete, rename), Download Station, Docker (7 namespaces) |
| `kwent/syno` | Node CLI | Unmaintained, DSM 5.x/6.x era | `fs`, `dl`, `dsm`, `ss`, `photo` subcommands, `~/.syno/config.yaml`, `--ignore-cert-errors`, per-API version override |
| `brendanSapience/Synology-DSM-Command-Line-Interface` | Python CLI | Thin | Named login sessions persisted to disk, File Station only |
| `N4S4/synology-api` | Python library | Active, broadest coverage | Many namespaces, QuickConnect, OTP - but a library, not a CLI |
| `hacf-fr/synologydsm-api` | Python library | Active, narrow | Home Assistant polling, device_token reuse for two-factor accounts |
| DSM web UI itself | Reference implementation | Live, sniffed this run | 33 unique api.method pairs captured with their real parameter names from an authenticated session on a live DS415+ |

## Absorbed (match or beat everything that exists)

| # | Feature | Best source | Our implementation | Added value |
|---|---------|-------------|--------------------|-------------|
| 1 | DSM login with session id | synology-mcp (source), DSM UI (sniff) | `session login` plus the framework's session_handshake credential store | `--otp-code` and `--device-id` both supported, so a two-factor account logs in once and stays trusted |
| 2 | Session expiry recovery | synology-mcp (source) | Error-119 detection with one transparent relogin and retry | Applies to every command, not just the ones the MCP wrapped |
| 3 | API namespace discovery | none of the competitors | `session apis` | Unauthenticated inventory of all 585 namespaces the NAS exposes, so a user can see what their DSM actually offers |
| 4 | System info, health, utilization | synology-mcp, N4S4 | `system info`, `system health`, `system utilization` | Three distinct commands instead of one blob; each is JSON-first and pipeable |
| 5 | Network configuration | synology-mcp | `system network` | - |
| 6 | Service list | synology-mcp | `system services` | - |
| 7 | UPS state | synology-mcp | `system ups` | - |
| 8 | Storage overview | synology-mcp, DSM UI (sniff) | `storage overview` | One call returns the whole Storage Manager picture |
| 9 | Volumes, pools, disks | synology-mcp, N4S4 | `storage volumes`, `storage pools`, `storage disks` | - |
| 10 | SMART attributes | synology-mcp | `storage smart --device sata1` | No competing CLI exposes SMART at all |
| 11 | SMART test schedule | DSM UI (sniff) | `storage smart-schedule` | Not present in any competitor or in the MCP |
| 12 | iSCSI LUNs | synology-mcp | `storage luns` | - |
| 13 | External USB and eSATA storage | DSM UI (sniff) | `storage usb`, `storage esata` | Not present in the MCP |
| 14 | Shared folders and permissions | synology-mcp | `folder list`, `folder get`, `folder permissions` | - |
| 15 | Users and their groups | synology-mcp | `user list`, `user get`, `user groups` | - |
| 16 | Groups and members | synology-mcp | `group list`, `group members` | - |
| 17 | NFS service state | synology-mcp | `nfs` | - |
| 18 | Installed packages | synology-mcp, DSM UI (sniff) | `package` | - |
| 19 | System log with filters | synology-mcp, DSM UI (sniff) | `log` | Full sniffed parameter set: logtype, level, keyword, date range, target, sort direction, paging |
| 20 | File Station browse | synology-mcp, kwent/syno | `files shares`, `files list`, `files stat`, `files info` | Sniffed parameter set including pattern, filetype and both sort parameters |
| 21 | File Station search with task polling | synology-mcp | `files search-start`, `files search-results`, `files search-stop` | The task lifecycle is exposed rather than hidden, so an agent can poll on its own schedule |
| 22 | Download a file | synology-mcp, kwent/syno | `files download` | - |
| 23 | Create folder, rename | synology-mcp | `files mkdir`, `files rename` | - |
| 24 | Copy, move, delete with task polling | synology-mcp | `files copy-start/copy-status/copy-stop`, `files delete-start/delete-status/delete-stop` | Same explicit task lifecycle as search |

## Transcended (what nobody else offers)

| # | Capability | Why it beats the field |
|---|-----------|------------------------|
| T1 | Multi-NAS through the framework's built-in `--profile` | Every competitor is single-NAS or needs a hand-edited config per host |
| T2 | Local store plus `sync` and offline `search` | DSM's own search re-scans the volume every time; no competing CLI caches anything |
| T3 | JSON-first output with `--agent`, `--compact`, `--select` | Every competitor prints human tables only |
| T4 | MCP server shipped alongside the CLI from one spec | The existing MCP and the existing CLIs are separate projects with different coverage |
| T5 | `session apis` inventory | Turns an undocumented, install-dependent API surface into something a user can enumerate |

## Deliberately not shipped in v1

| Feature | Source that has it | Why excluded |
|---------|--------------------|--------------|
| Container Manager: container list/inspect/logs/start/stop/restart, image list/prune/pull, project up/down/rebuild, network, registry (7 `SYNO.Docker.*` namespaces) | synology-mcp | `SYNO.Docker.*` is absent from `SYNO.API.Info?query=all` on the target NAS - the Container Manager package is not installed. There is no live evidence and no way to behaviourally test these commands from this machine. |
| Download Station: task list/create/pause/resume/delete, statistics, downloaded files (`SYNO.DownloadStation*`, `SYNO.DownloadStation2*`) | synology-mcp | Same reason: the Download Station package is not installed, so the namespaces do not exist on this NAS. |
| Mutating user, group, share and NFS operations (create, set, delete, permission changes) | synology-mcp | Every one of them is destructive on a live production NAS and cannot be smoke-tested safely. The read surface for all four areas ships in full. |

The two package-gated areas are the whole reason for `session apis`: once the packages are installed, the inventory command shows the namespaces appear, and a follow-up run can absorb them with live evidence behind them.
