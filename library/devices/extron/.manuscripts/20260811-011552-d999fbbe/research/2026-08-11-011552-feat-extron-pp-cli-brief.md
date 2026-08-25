# Extron CLI Brief

## API Identity
- Domain: Pro AV control — driving Extron signal processors (matrix switchers, amps, scalers, media processors, power control interfaces) over the network from a fleet perspective
- Users: AV integrators and field techs, system programmers (ControlScript/Global Scripter crowd), facilities AV operators, Home Assistant / automation power users, AI agents doing AV ops
- Data profile: LAN devices (host/IP + port), per-device credentials, per-model SIS command vocabularies, polled status snapshots, fleet-wide health. No cloud API, no API keys — the "API" is the device on the user's AV network

## Reachability Risk
- Low (protocol): SIS is a mature ASCII text protocol; multiple independent community clients work today (pyextron, ha-extron, sismatic, ioBroker.extron, Extron-Telnet-Control). No 403/broken/blocked reports on ha-extron issues — all 30 issues are maintenance/dependency updates.
- The API is LAN-only: no stable global origin to probe from the generation host (Phase 1.9 carve-out: `lan-only-no-global-url`). extron.com marketing site reset the curl connection and manualzz mirror 403s bots — neither affects the device contract.
- Transports (all user-supplied device addresses):
  - SIS over TCP port 23 (telnet-style, plaintext) — classic IP Link era
  - SIS over SSH port 22023 (modern IPCP Pro era; default creds `admin`/`extron` per sismatic docs)
  - SIS over HTTP via IP Link web server: query strings (`index.html?cmd=<sis>`) and SSI (`<!--#echo var="<sis-command>"-->`)
  - RS-232 serial (out of scope for a network CLI)

## Top Workflows
1. Power control — turn a display/projector on or off via a power control interface (PCS4, IPL T) or matrix output; check power state
2. Source routing — switch input N to output/zone M on a matrix switcher (MAV/DTP/DVS families); query current route
3. Audio control — set/step volume, mute/unmute per zone on amps and surround processors (MAV88 pattern: `{zone}*{vol:02}V`, `{zone}*1Z`/`{zone}*0Z`, status query `{zone}${zone}Z{zone}V`)
4. Status & health — device identity banner (model, firmware, part number, date/time), input/output signal status, zone status; fleet `doctor` pass
5. Fleet batch operations — push the same command to many devices (PardosTechSamples SMD batch; sismatic device pool), keep a local device registry

## Table Stakes (competitor features — must be matched)
- pyextron (zombielinux/pyextron, PyPI): telnet SIS client; per-series YAML protocol config (command templates, EOL `\r\n`, limits); zone status regex parsing; volume/mute/source commands; min-time-between-commands rate limiting (0.4s MAV88)
- sismatic (PyPI, 0.2.18): SIS over SSH device pool; TOML/JSON/YAML device registry with per-device overrides (port 22023, connect_secs, command_secs); lazy connections + keepalive; batch commands; metadata registers; eager-connect option
- ha-extron (NitorCreations, Home Assistant): connection lifecycle management; unavailable-on-unreachable; user-defined input names; 30s polling; diagnostics sensors (HDMI/HDCP)
- ioBroker.extron: JS SIS adapter with polling
- adnbr/Extron-Telnet-Control: dead-simple CLI power on/off over telnet
- PardosTechSamples/ExtronSetups: batch volume + multicast config for SMD media processors
- Status quo: PuTTY / netcat / per-device Python scripts

## Data Layer
- Primary entities: `devices` (registry: name, host, port, transport, username/password ref, model, zone/source names), `commands` (built-in SIS catalog per device family), `status` (per-device snapshot: power, routes, volume/mute, signal, firmware — timestamped)
- Sync cursor: per-device status snapshot polling (HA uses 30s; CLI `sync` polls the registry)
- FTS/search: search device registry and the command catalog

## Product Thesis
- Name: `extron-pp-cli`
- Why it should exist: today, driving an Extron fleet means PuTTY sessions, per-device scripts, or bolting Home Assistant to one room. A single CLI with a device registry, a SIS command catalog, structured JSON output, batch `exec`, and `doctor` health checks gives integrators *and agents* one tool for the whole fleet — with offline query over the local status store.

## Build Priorities
1. Device registry + per-device auth (add/list/remove devices; username/password per device)
2. SIS transport: TCP (23) + SSH (22023) clients; raw `send <device> <command>` with response capture; error-code decoding (E01–E28)
3. High-level device-family commands: `power`, `route`, `volume`, `mute`, `status` (MAV/switcher/IPL shapes from the catalog)
4. Command catalog: built-in SIS vocabulary per family + `catalog list`/`catalog show`
5. `doctor` fleet health check + `exec` batch across devices
6. HTTP SSI/query-string transport as an alternate path for IP Link devices
