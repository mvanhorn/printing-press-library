# Crowd-sniff report: unifi
- Command: crowd-sniff --api unifi --base-url https://api.ui.com. Without --base-url it exits 4 (base URL undeterminable), since UniFi's controller API has no public host.
- Result: 75 resources; tier breakdown code-search 64, community-sdk 53 (see crowd-sniff-provenance.json).
- Competitor source read directly: enuno/unifi-mcp-server (62 tool modules, 69 non-integration controller paths; see below).
- Auth: X-API-KEY everywhere. enuno's local client uses `ea/sites/...` aliases and `proxy/network/api/s/...`, `proxy/network/v2/api/site/...`.
## Controller paths proven by competitor source
rest/: account, dynamicdns, firewallgroup, firewallrule, hotspotpackage, networkconf (+references), portconf, portforward, radiusprofile, tag, user, wlanconf.
stat/: alarm, alluser, device, event, report/*, rogueap, sitedpi, spectrum-scan, stadpi, backup.
cmd/: backup, devmgr, stamgr. get+set/setting: auto_backup, dpi, guest_access, ips, super_mgmt.
v2: ap/{id}/neighbors, system-log/all, system-log/critical, traffic-flows. Backup: delete-backup, restore.
## Coverage
Cross-confirms the bundle list in browser-sniff-report.md. Live proof through the connector still needed.
