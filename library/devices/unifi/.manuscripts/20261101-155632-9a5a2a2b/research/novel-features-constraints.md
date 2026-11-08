# Novel feature constraints (from the adversarial cut; binding on the build)

## reach (hand-code, pp:data-source live)
Per console: which transports (connector, local) and surfaces (site-manager, network, protect, innerspace, controller classic) answer, and write status yes|no|unproven.
- Uses /v1/hosts plus one read per surface.
- Write probe opt-in (--probe-writes) and non-mutating, e.g. a mutating verb against an all-zero UUID. A 404 may come from the proxy before auth, so anything ambiguous reports `unproven`, never `yes`.
- Report configured VPN servers and WAN count as second-path facts; configured does not mean tested.
- Reports which connector path shape works (spec conflict: `/proxy/network/integration/...` per Network spec servers vs `/network/integration/...` per Site Manager example).

## impact (hand-code, local, drain-first follow-ups)
Local join: devices (uplink.deviceId), clients (uplinkDeviceId, macAddress), Protect cameras (mac), WiFi broadcasts, WANs, firewall policies.
- Verdicts: blocks-path, at-risk, no-known-risk, cannot-determine. NEVER "safe".
- v11 gives parent device but no parent port index, so port power-cycle returns cannot-determine unless classic data is available.
- Models connector dependence on gateway uplink; state it cannot see every cause of path loss.

## change apply / change undo (hand-code, live)
GET before-image, official mutation endpoint, read-back, local journal. Foreground only, no daemon.
- Journal commits BEFORE any store upsert.
- Value requires writes; docs say cloud keys are read-only today. Must fail clearly when writes are refused.
- Auto revert only if the rollback host still has a path. If path dies: cannot revert, exit non-zero, journal `unresolved`.
- --trial: new firewall policies only; uses POST firewall policies with ONE_TIME_ONLY schedule so the gateway stops enforcing. Refuse on edits to existing policies. Schema takes one date and HH:MM with no timezone: needs console-local time, appears unable to cross midnight. Expiry behavior unverified live; say so.
- Undo limits: restart and power-cycle cannot be undone; recreated objects get new IDs that can break references; undo refuses if current state differs from the journal's after-image.
- Ambiguous writes: after 408 read back, never replay.

## digest (hand-code, local, hintIfUnsynced "" , drain-first)
Mirror snapshots of hosts, devices, clients, cameras, plus isp metrics and the journal, into one time-ordered list per console. Transitions seen only at sync points; a flap between syncs is missed. Site Manager lastConnectionStateChange partly offsets for hosts and devices. Do not assert causal ISP blame.

## drift (hand-code, local)
Two console-keyed snapshots of networks, firewall, WiFi, DNS; field-level changes marked attributed (in journal) or out-of-band. Reject live-only requests before touching local state (prior patch).

## newcomer (hand-code, local)
Local first-seen on console-keyed rows. Only clients present at a sync are seen. MAC randomization may inflate results; note it.

## rule-predict (hand-code, local)
Zones, ordered policies, ACL rules, TMLs; first matching policy or `indeterminate`. Domain, application, region filters return indeterminate. Legacy firewall rules outside the model unless classic path proven. Output must flag it is a prediction with no API confirmation.

## topology (hand-code, local)
Parent-child tree per console from uplink.deviceId and client uplinkDeviceId. Shared with impact.
