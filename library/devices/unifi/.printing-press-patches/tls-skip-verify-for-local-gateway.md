# Patch: TLS skip-verify for local UniFi gateway

## Files touched
- `internal/config/config.go` — added `InsecureSkipVerify()` + `isPrivateNetworkHost()`, and a `UNIFI_GATEWAY_HOST` → `BaseURL` (`https://<host>/proxy/network`) default construction since the integration spec's `servers[0].url` is a bare relative path (`/integration`) with no absolute host.
- `internal/client/client.go` — `newHTTPClient` now takes an `insecureSkipVerify bool` and sets `tr.TLSClientConfig.InsecureSkipVerify` when true; `New()` passes `cfg.InsecureSkipVerify()`.

## Why
Self-hosted UniFi OS gateways (UDM/UCG/UDR-class) serve a self-signed certificate on the local Network API by default. Every existing community tool for this API (`unifi-cli`, `mcp-unifi`) ships a manual `--insecure`/env-var opt-out because of this. The Printing Press generator has no TLS-skip-verify support at all today — a freshly generated CLI cannot talk to a real gateway out of the box.

## Behavior
- Auto-skips verification only when the resolved `BaseURL` host is loopback, RFC1918/RFC4193 private, or link-local (RFC3927) — i.e., the gateway is being reached on the user's own LAN, which is the only way this API is ever exposed.
- `UNIFI_INSECURE_SKIP_VERIFY=1|true|yes|on` / `=0|false|no|off` overrides the auto-detection explicitly in either direction.
- A DNS hostname (not a literal IP) is never treated as private, even if it happens to resolve to a private address — resolving it at config-load time would add a network round-trip and a DNS-spoofing surface. Users reaching their gateway by a non-private hostname must set the env override explicitly.

## Generator gap this closes
No TLS-skip-verify support exists anywhere in the generator's client/config templates. This is not UniFi-specific — any CLI printed for a LAN-appliance API (routers, NAS boxes, home-automation hubs, other self-hosted gear) will hit the same wall. Worth a `/printing-press-retro` filing so the generator gains first-class support and future prints don't need this hand patch.

## Reprint guidance
If this CLI is regenerated from a fresh `cli-printing-press generate`, this patch is NOT carried automatically — re-apply both edits (or check whether the generator has since gained native TLS-skip-verify support, in which case drop this patch and use the native mechanism).
