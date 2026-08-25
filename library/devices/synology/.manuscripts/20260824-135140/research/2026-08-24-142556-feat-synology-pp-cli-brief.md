# Synology DSM CLI Brief

## API Identity
- Domain: Synology DSM WebAPI (`/webapi/entry.cgi`, `/webapi/auth.cgi`) on a LAN NAS. Namespaced RPC-over-HTTP, not REST: every call is `api=SYNO.X.Y&method=z&version=N` with `_sid` and `X-SYNO-TOKEN`.
- Users: homelab and SMB NAS operators, sysadmins, backup/media pipeline automation.
- Data profile: files and shares, download tasks, Docker containers/images/projects, users/groups, storage pools/volumes/disks/SMART, system utilization, syslog, NFS exports, UPS, iSCSI LUNs.

## Reachability Risk
- Low, with one caveat. The API is local and authenticated; there is no public rate limit and no anti-bot layer. Caveat: no official OpenAPI spec exists. Synology publishes only partial PDFs (File Station, Download Station, Surveillance Station); `SYNO.Core.*` and `SYNO.Docker.*` are undocumented and reverse-engineered.
- Tier/permission hints: DSM error 119 = SID expired (needs transparent relogin), 105 = insufficient permission, 403/404 in the `error.code` field, never in HTTP status - DSM returns HTTP 200 with `success:false`.
- Probe-safe endpoint: `GET /webapi/query.cgi?api=SYNO.API.Info&method=query&version=1&query=all` - unauthenticated, enumerates every installed API with its min/max version and CGI path. This is the machine-readable spec surface.

## Source Priority
Single source. Primary spec input is the locally installed `synology-mcp` v0.1.0 package (`%APPDATA%/uv/tools/synology-mcp/Lib/site-packages/{auth,container,downloadstation,filestation,health,nfs,usermanagement,utils}`), 3444 lines of working, param-exact Python against a live DSM. Secondary: `SYNO.API.Info?query=all` from the live NAS. Tertiary: community docs (N4S4/synology-api, pmilano1/synology-dsm-api).

## Codebase Intelligence (from installed synology-mcp)
- Auth: `SYNO.API.Auth` v6/v7 login, returns `sid` + `synotoken`. `X-SYNO-TOKEN` header required on mutating calls from DSM 7.3.2+. OTP via `otp_code`. Session expires ~1h idle for `SYNO.Core.*`; error 119 must trigger one transparent relogin + retry, with concurrent recoveries collapsing onto a single relogin.
- Multi-NAS: config registry at `~/.config/synology-mcp/settings.json` under `synology.<name>` with host/port/username/password/note. A CLI must keep this profile model (named NAS, `--nas` selector) - the local install already has one profile registered.
- API surface actually exercised (39 namespaces): `SYNO.API.Auth`, `SYNO.DSM.Info`, `SYNO.Core.{System,System.Utilization,Network,Package,Share,Share.Permission,User,User.Group,Group,Group.Member,Storage.Disk,Storage.Pool,Storage.Volume,SyslogClient.Log,FileServ.NFS,ExternalDevice.UPS,ISCSI.LUN}`, `SYNO.Storage.CGI.{Storage,Smart}`, `SYNO.FileStation.{List,Search,Download,CopyMove,CreateFolder,Delete,Rename}`, `SYNO.DownloadStation.{Info,Statistic}`, `SYNO.DownloadStation2{,.Task}`, `SYNO.Docker.{Container,Container.Log,Container.Resource,Image,Network,Project,Registry}`.
- Transport quirks: all params go as query string even on POST-style calls; `verify_ssl` is off by default because DSM ships a self-signed cert on 5001; long-running FileStation ops (search, copy/move, delete) are taskid-based and must be polled.

## Top Workflows
1. Fleet health check: one command that returns system info, utilization, volume/pool state, disk SMART, UPS, and recent syslog errors across every configured NAS.
2. Container operations: list/inspect/restart containers, tail logs, prune images, start/stop/rebuild compose projects - the DSM Container Manager UI is slow and click-heavy.
3. Storage/SMART triage: find the failing disk, its pool and volume, its bad-sector trend, before the NAS emails about it.
4. File Station bulk work: search across shares, copy/move/delete with task polling, fetch a file's content into a pipe.
5. Download Station queue management: create/pause/resume/delete tasks, statistics, list finished files.

## Table Stakes (what competitors ship)
- `kwent/syno` (Node): subcommand-per-service (`fs`, `dl`, `dsm`, `ss`, `photo`), `~/.syno/config.yaml`, `--ignore-cert-errors`, per-API version override. Closest competitor. Unmaintained, DSM 5.x/6.x era, no DSM 7 CSRF token, no Docker/Container Manager, no SMART.
- `brendanSapience/Synology-DSM-Command-Line-Interface` (Python): named login sessions persisted to disk. Thin, File Station only.
- `N4S4/synology-api` (Python lib): broadest namespace coverage, QuickConnect support, OTP - but a library, not a CLI, and no local store or search.
- `hacf-fr/synologydsm-api`: Home Assistant polling lib, device_token reuse for 2SA. Narrow.
- Nothing in the field offers: multi-NAS fleet in one invocation, a local SQLite store, offline search, or agent-native JSON output.

## Data Layer
- Primary entities: nas (profile), container, image, project, share, file (indexed subset), download_task, user, group, volume, pool, disk, smart_sample, syslog_entry, utilization_sample.
- Sync cursor: per-entity `last_synced_at`; syslog by log id/timestamp; utilization and SMART are time series - append samples so `disk trend` and `util history` work offline.
- FTS/search: FTS5 over file paths/names from `SYNO.FileStation.Search` results, over container names/images/labels, and over syslog message text. This is the single biggest win: DSM's own search is slow and re-scans the volume every time.

## User Vision
None volunteered at briefing; the argument was "das installierte synology mcp", i.e. print a CLI whose capability floor is the installed MCP server's tool surface.

## Product Thesis
- Name: `synology-pp-cli` (binary), library slug `synology`.
- Why it should exist: every existing Synology CLI is a thin single-NAS RPC wrapper stuck on DSM 6. This one treats a NAS fleet as one queryable dataset - authenticated once per profile, synced into local SQLite, searchable offline, JSON-first for agents - and covers the DSM 7 surface (Container Manager, SMART, storage pools, NFS, UPS, LUNs) that no competing CLI touches.

## Build Priorities
1. Auth + profile core: `~/.config/synology-pp-cli/`-style multi-NAS profiles, login with OTP, sid+synotoken, error-119 transparent relogin, self-signed cert handling. Nothing works without this.
2. Health/storage read surface: `health`, `util`, `volume`, `pool`, `disk`, `smart`, `ups`, `syslog` - highest daily value, zero destructive risk, easiest to smoke-test live.
3. Container Manager: `container ls/inspect/logs/start/stop/restart`, `image ls/prune`, `project ls/up/down/rebuild`. The competitive gap.
4. File Station: `ls`, `search`, `cat`, `cp`, `mv`, `rm`, `mkdir` with taskid polling and a progress-safe `--wait`.
5. Download Station: `dl ls/add/pause/resume/rm/stats`.
6. Local store + FTS + `sync`: everything above cached, `search` across files/containers/syslog offline.
7. Fleet fan-out: `--nas all` on every read command, results tagged by profile.

## Reachability Gate
- Decision: PASS
- Reason: live-2xx-json
- Evidence: `GET http://<nas-host>:5000/webapi/query.cgi?api=SYNO.API.Info&method=query&version=1&query=SYNO.API.Auth` returned HTTP 200 with `{"data":{"SYNO.API.Auth":{"maxVersion":7,"minVersion":1,"path":"entry.cgi"}},"success":true}`. Plain HTTP, no challenge page, no bot protection, no browser sidecar needed. The spec's `base_url` is the placeholder `https://nas.example.com:5001` because DSM is a per-user LAN appliance; users point it at their own NAS through the profile config.
