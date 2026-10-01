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
| 10 | Policy file get (HuJSON or JSON, details) | YawLabs get_acl; tscli get policy | (generated endpoint) acl get | ETag surfaced; safe editing below |
| 11 | Policy file set with If-Match | YawLabs update_acl; tscli set policy | (generated endpoint) acl set | wrapped by policy commands with backup + diff |
| 12 | Policy validate / run tests | YawLabs validate_acl; tscli get policy validate | (generated endpoint) acl validate | validate always runs before safe writes |
| 13 | Policy preview rule matches | YawLabs preview_acl; tscli policy preview | (generated endpoint) acl preview | |
| 14 | Diff ACL access (who gains/loses) | YawLabs tailscale_diff_acl_access | (behavior in tailscale-pp-cli policy diff) line diff of HuJSON before any write | |
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
| 27 | Retry 429/5xx on idempotent methods only | YawLabs | (behavior in tailscale-pp-cli devices list) generated client retry/backoff | POST never retried |
| 28 | Debug HTTP logging without auth headers | YawLabs TAILSCALE_DEBUG; tscli --debug | (behavior in tailscale-pp-cli doctor) generated --verbose/debug | tokens redacted |
| 29 | Surgical policy add/remove of named blocks | scurgery apply/remove/diff | (behavior in tailscale-pp-cli policy add-entry) comment-preserving JSON-patch insert | adds validate + ETag + backup + restore |
| 30 | Device key-expiry report from local status | tailscale-superpowers check-all-macs.py | tailscale-pp-cli devices expiry | uses admin API: covers every device, not only peers visible locally |
