<!-- slop-gate: off -->
# Tailscale CLI Absorb Manifest

Sources: tscli (jaxxstorm/tscli, Go CLI), @yawlabs/tailscale-mcp (97 admin API tools), scurgery (nopoz/scurgery, surgical policy edits), tailscale/tailscale-skill (official reference skill), tailscale-superpowers check-all-macs.py (expiry report from local status).

## Absorb Manifest

### Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | List devices (default/all fields, server-side filters) | YawLabs MCP tailscale_list_devices; tscli list devices | (generated endpoint) devices list | --json/--select/--csv, `fields=all`, local store + search |
| 2 | Get / delete device | tscli get device; YawLabs get/delete_device | (generated endpoint) device get / device delete | delete honors --dry-run, agent-mode dry-run default |
| 3 | Authorize / deauthorize device | YawLabs authorize/deauthorize_device | (generated endpoint) device authorized | --dry-run |
| 4 | Rename, tags, IP, key settings, expire key | YawLabs rename/set_device_tags/update_device_key/expire_device | (generated endpoint) device name / tags / ip / key / expire | --dry-run, typed exit codes |
| 5 | Get / set device routes | tscli list routes / set device routes; YawLabs get/set_device_routes | (generated endpoint) device routes | raw replace kept; safe wrappers below |
| 6 | Device posture attributes get/set/delete, batch update | YawLabs posture tools; tscli | (generated endpoint) device attributes | --dry-run |
| 7 | Device invites (share) list/create/get/delete/resend/accept | YawLabs 6 invite tools; tscli list/delete invites | (generated endpoint) device-invites | cross-device share audit below |
| 8 | User invites list/create/get/delete/resend | YawLabs; tscli | (generated endpoint) user-invites | --dry-run |
| 9 | Users list/get/role/approve/suspend/restore/delete | YawLabs 7 user tools; tscli | (generated endpoint) users | local search |
| 10 | Policy file get (HuJSON or JSON, details) | YawLabs get_acl; tscli get policy | (generated endpoint) tailnet acl get | ETag surfaced; safe editing below |
| 11 | Policy file set with If-Match | YawLabs update_acl; tscli set policy | (generated endpoint) acl set | wrapped by policy commands with backup + diff |
| 12 | Policy validate / run tests | YawLabs validate_acl; tscli get policy validate | (generated endpoint) acl validate | validate always runs before safe writes |
| 13 | Policy preview rule matches | YawLabs preview_acl; tscli policy preview | (generated endpoint) acl preview | |
| 14 | Diff ACL access (who gains/loses) | YawLabs tailscale_diff_acl_access | (behavior in tailscale-pp-cli policy add-entry) line diff of HuJSON before any write | |
| 15 | DNS nameservers/preferences/searchpaths/split-dns/configuration | YawLabs 11 DNS tools; tscli | (generated endpoint) dns | --dry-run on writes |
| 16 | Keys list/get/create/delete/set (auth keys, OAuth clients, federated) | YawLabs 9 key tools; tscli create key | (generated endpoint) keys | secret only printed at creation, never logged |
| 17 | Tailnet settings get/patch | YawLabs; tscli set settings | (generated endpoint) settings | --dry-run |
| 18 | Contacts get/update/resend | YawLabs; tscli | (generated endpoint) contacts | |
| 19 | Webhooks list/create/get/update/delete/test/rotate | YawLabs 7; tscli | (generated endpoint) webhooks | |
| 20 | Posture integrations CRUD | YawLabs 5; tscli | (generated endpoint) posture | |
| 21 | Services (VIP) list/get/update/delete/hosts/approval | YawLabs 7; tscli | (generated endpoint) services | |
| 22 | OAuth apps CRUD | YawLabs | (generated endpoint) oauth-apps | |
| 23 | Organization tailnets list/create, delete tailnet | YawLabs 3 | (generated endpoint) organizations | |
| 24 | Log streaming config/status, AWS external ID | YawLabs 7; tscli get logs aws | (generated endpoint) logging | |
| 25 | Configuration audit log and network flow logs | YawLabs get_audit_log / network_flow_logs | (generated endpoint) logging configuration / network | |
| 26 | API token and OAuth client-credentials auth | YawLabs (TAILSCALE_API_KEY / TAILSCALE_OAUTH_CLIENT_ID/SECRET); tscli profiles | (behavior in tailscale-pp-cli auth) env + config, OAuth token exchange | doctor reports which auth path is live |
| 27 | Retry 429/5xx on idempotent methods only | YawLabs | (behavior in tailscale-pp-cli tailnet devices list) generated client retry/backoff | POST never retried |
| 28 | Debug HTTP logging without auth headers | YawLabs TAILSCALE_DEBUG; tscli --debug | (behavior in tailscale-pp-cli doctor) generated --verbose/debug | tokens redacted |
| 29 | Surgical policy add/remove of named blocks | scurgery apply/remove/diff | (behavior in tailscale-pp-cli policy add-entry) comment-preserving JSON-patch insert | adds validate + ETag + backup + restore |
| 30 | Device key-expiry report from local status | tailscale-superpowers check-all-macs.py | tailscale-pp-cli devices expiry | uses admin API: covers every device, not only peers visible locally |

### Transcendence (only possible with our approach)
| # | Feature | Command | Score | Buildability | How It Works | Evidence | Long Description |
|---|---------|---------|-------|--------------|--------------|----------|------------------|
| 1 | Route approval that keeps other routes | routes approve | 10/10 | hand-code | GET /device/{id}/routes, union requested CIDRs (or the exit-node pair) into enabledRoutes, refuse unadvertised CIDRs, re-read before POST, verify after | User Vision lesson 2; tscli/YawLabs only expose raw replace | Use this command to approve subnet routes or an exit node on one device while keeping its other approved routes. Do NOT use this command to find which routes are waiting for approval across the tailnet; use 'routes overview' instead. |
| 2 | Route unapproval that keeps other routes | routes unapprove | 10/10 | hand-code | Same read-modify-write as approve, sending enabledRoutes minus the requested CIDRs | User Vision lesson 2 (remove exit node pair 0.0.0.0/0 + ::/0) | Use this command to remove approval for specific routes or the exit node on one device while keeping the rest. Do NOT use this command to find pending routes; use 'routes overview' instead. |
| 3 | Tailnet route board | routes overview | 9/10 | hand-code | GET /tailnet/-/devices?fields=all; per device approved, pending (advertised minus enabled), stale (enabled minus advertised), exit-node state | Mid-run follow-up: advertised but unapproved routes/exit nodes | Use this command to see advertised, approved, pending, and stale routes and exit-node state across devices. Do NOT use this command to change approvals; use 'routes approve' or 'routes unapprove' instead. |
| 4 | Fleet key-expiry report | devices expiry | 10/10 | hand-code | GET /tailnet/-/devices (+ /keys?all=true with --include-keys); days left, --within window, non-zero exit when any item is inside it | User Vision lesson 4; check-all-macs.py only saw local peers | Use this command for a fleet-wide key-expiry report or a cron/CI expiry gate. Do NOT use this command for the full state of one device; use 'devices inspect' instead. |
| 5 | Safe policy entry insert | policy add-entry | 10/10 | hand-code | HuJSON GET + ETag, local backup, hujson JSON Patch append to nodeAttrs/grants, /acl/validate, line diff, POST with If-Match | User Vision lesson 1; scurgery lacks validate/ETag/backup | Use this command to append one nodeAttrs or grants entry to the live policy file. Do NOT use this command to roll the policy back to an earlier version; use 'policy restore' instead. |
| 6 | Policy rollback | policy restore | 8/10 | hand-code | Reads local backup, takes a pre-restore backup, validates, diffs current vs backup, POST with If-Match on current ETag; --list shows backups | User Vision requests policy restore <backup> | Use this command to roll the live policy file back to a local backup or to list backups. Do NOT use this command to add a new entry; use 'policy add-entry' instead. |
| 7 | Machine share audit | shares audit | 10/10 | hand-code | GET devices, then GET /device/{id}/device-invites per device; one table with acceptor, state, multiUse, allowExitNode; flags redeemable pending invites; inviteUrl redacted by default | User Vision share list; admin console shows shares per device only | Use this command to list machine-share invites across the tailnet, filtered by device, state, or acceptor. Do NOT use this command for the full state of one device; use 'devices inspect' instead. |
| 8 | Machine share revoke | shares revoke | 10/10 | hand-code | Resolves invite IDs (by ID or --device/--accepted-by filters) and calls DELETE /device-invites/{id}, plan first | User Vision share revoke | Use this command to revoke machine-share invites by ID or by device/acceptor filter. Do NOT use this command to audit shares; use 'shares audit' instead. |
| 9 | Reachability preview | access check | 9/10 | hand-code | Resolves device to Tailscale IP, posts the live (or --policy-file) policy to /acl/preview for type=ipport and type=user, intersects matches by line | Mid-run follow-up: which device can reach which | Use this command to check which policy rules let a user reach a device and port, against the live policy or a candidate file. Do NOT use this command to change the policy; use 'policy add-entry' instead. |
| 10 | Device dossier | devices inspect | 9/10 | hand-code | Resolves self/hostname/nodeId/status-ID/IP (local status ID == nodeId), reads device (fields=all), device invites, last 7 days of audit events targeting it | Mid-run follow-up: resolve local IDs, who has access, what changed | Use this command to get one device's identity, routes, expiry, shares, and recent changes in a single record before acting on it. Do NOT use this command for fleet-wide lists; use 'devices expiry', 'routes overview', or 'shares audit' instead. |

### Intentionally omitted
- DELETE /tailnet/{tailnet} (delete an API-only tailnet). The generator promotes a single-endpoint resource onto its parent command, so a bare `tailscale-pp-cli tailnet` would have sent DELETE /tailnet/-. Dropped from the spec; use the API directly for this rare org-admin action. Upstream generator issue to be filed.

### Stubs
None.

### Gate decision
Cathryn pre-authorized autonomous execution on 2026-09-30 ("create a CLI, novel commands for agent friendly networking ... create a pr when confident and dogfood everything") and stepped away. Gate 1.5 recorded as Approve with the full manifest; no stubs.
