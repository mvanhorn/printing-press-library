# Browser-sniff report: unifi.ui.com

Target: logged-in Chrome (chrome-MCP, fresh tab, closed afterward). Read-only: pages viewed, no settings changed.
Observed console: UDM SE, UniFi OS 5.1.34, Network 10.6.109, apps Network, Protect, Access, Talk.

## Findings
1. Console app traffic is not visible as HTTP. Dashboard, Topology and Devices rendered live data while fetch and XHR interceptors and the network log recorded only static assets from net-fe-static-assets.network-controller.svc.ui.com. Inference: the browser reaches the console over a tunnel, likely WebRTC (cloudaccess /create-credentials precedes it). Not directly proven.
   Consequence: captured traffic cannot supply console endpoints. Not replayable; no browser transport will ship.
2. Site-level cloud backend seen (paths only, session auth): cloudaccess.svc.ui.com /api/v1/hosts, /api/v1/sites, /api/v1/devices, /api/v1/admins, /invitations, /colocated-hosts, /vantage-points, /network-cloud/v2/sd-wan/configs, /create-credentials; sso.ui.com /api/sso/v1/user/self, /console_groups; enterprise.svc.ui.com /api/v2/organizations; fabric.svc.ui.com msp endpoints; ues.svc.ui.com subscriptions; ips.svc.ui.com subscriptions. Hosts, sites, devices and SD-WAN duplicate the official API. Replay needs a browser session; not attempted.
3. Controller endpoints from public JS bundles (bundle-paths.txt; discovered: bundle, not traffic): rest/ (user, usergroup, device, portforward, routing, firewallgroup, dhcpoption, dpiapp, hotspotpackage, hotspotop, generic rest/{x}), stat/ (health, device, device-basic, sysinfo, report, rogueap, spectrum-scan, sdn, guest, voucher, portforward, current-channel, firmware, widget/warnings), cmd/ (devmgr, devmgr/speedtest, stat, firewall, hotspot, backup, cfgmgr, sitemgr, system, qos-rules, lte, mbb), set/setting, /v2/api site routes.
   These correspond to what competitors expose and the official specs lack.

## Not verified, needs an API key
- Whether `X-API-KEY` works on classic controller paths through the connector. Third-party note reports yes (git.azcomputerguru.com, unverified); jason-mspkickstart/mcp-unifi reads health and clients that way.
- Whether a cloud key can write through the connector. Official docs say keys are read-only today.
- The correct connector path shape, since the spec contradicts itself.

## Replayability verdict
- Official specs: replayable with API key.
- Controller paths: plausibly replayable via connector or local; gated on live proof, and read-only proof must come first.
- Private cloudaccess and sso surfaces: session-only; recommend excluding.
- Access: UI present, no official developer.ui.com spec; pete-builds exposes 18 Access tools via another path. Talk: no API found.
