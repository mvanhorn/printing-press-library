# Synology Router API Research

## Overview
Synology Router Manager (SRM) provides a REST-like API via `/webapi/` CGI endpoints.
Authentication uses session-based cookies obtained through `auth.cgi`.

## Key Endpoints
- `SYNO.Core.Network.NSM.Device` — Device management
- `SYNO.Core.NGFW.Traffic` — Traffic monitoring
- `SYNO.Core.Network.Router.PolicyRoute` — Firewall rules
- `SYNO.Mesh` — Mesh network management
- `SYNO.Core.Network.SmartWAN` — Multi-WAN configuration
- `SYNO.Core.Network.WOL` — Wake-on-LAN

## Authentication
SRM uses form-encoded POST to `auth.cgi` with `api=SYNO.API.Auth&method=Login&version=2`.
Session cookie is `id` returned in response data.
