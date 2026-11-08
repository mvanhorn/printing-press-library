---
name: pp-unifi
description: "Operate UniFi consoles through the cloud connector or your LAN, with a local history, change previews and a change journal. Trigger phrases: `check my unifi network`, `what changed on my network`, `restart my access point`, `what would restarting this unifi device affect`, `use unifi`, `run unifi`."
author: "Ricardo Cabral"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - unifi-pp-cli
    install:
      - kind: go
        bins: [unifi-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/devices/unifi/cmd/unifi-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/devices/unifi/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# UniFi: Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `unifi-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install unifi --cli-only
   ```
2. Verify: `unifi-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.10 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/devices/unifi/cmd/unifi-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

One binary for the official UniFi APIs, over the cloud connector or a direct connection to your gateway. A local mirror keeps a snapshot from each sync, and 'impact' estimates what a change would affect before you make it. Reads have run against one test console over the cloud connector; most writes have not.

## When to Use This CLI

Use for operating UniFi consoles over the cloud connector or a local gateway: health, config review, device and guest actions, Protect, and what-changed questions from local snapshots. Run 'impact' before, and prefer 'change apply' for, changes that could affect connectivity. Start with reads: cloud writes are unproven and no route has been tested live.

## Anti-triggers

Do not use this CLI for:
- UniFi Access or Talk administration: no official API here and no commands cover them
- Blocking a client with an official command: the API offers only guest authorize and unauthorize, so blocking means an unverified raw controller path or a firewall or ACL rule
- Proving a firewall outcome: rule-predict is a prediction with no API confirmation
- Live video streaming beyond handing out RTSPS links
- First-time console setup and account administration

## Before You Act

Live testing so far covers reads against one test console over the cloud connector, plus one hotspot voucher created and deleted there. InnerSpace, Mobility and Carrier refused that key. Everything else, including most writes, rests on mock servers and `--dry-run`. Report results as observed only when you have seen them.

- **Writes:** docs say cloud keys are read-only today, yet one hotspot voucher create and delete worked with a cloud key on one test console. Other cloud writes are unproven. Run `reach` first and expect that a write through the connector may be refused. Local writes with a local key are untested live.
- **Most writes send immediately:** `network`, `protect`, `carrier` and `mobility` write commands and `import` have no confirmation step, and `--agent` adds none. Use `--dry-run` first. `raw`, `change apply` and `change undo` preview until `--confirm`.
- **`raw`:** controller paths are unofficial and unverified. A `raw` write has no journal or undo.
- **`change apply --trial`:** expiry is unverified, so check the policy after the window. Trials cannot cross midnight, and times must be console-local.
- **`rule-predict`:** a prediction with no API confirmation. Do not report it as what the gateway will do.
- **`impact`:** reasons from the last sync and never means safe. `no-known-risk` is not a guarantee.
- **Local history:** `drift`, `digest`, `newcomer`, `topology`, `impact` and `rule-predict` read the mirror, so they are only as current as the last `sync`.
- **Not covered:** UniFi Access and Talk.

## Unique Capabilities

### Safe remote operation
- **`reach`** — See which transports and products answer on each console. Write status is yes, no or unproven, and a probe alone never reports yes.

  _Run this first in a session to learn what answers before planning a change. Spends one read per surface per console._

  ```bash
  unifi-pp-cli reach --agent
  ```
- **`impact`** — Estimate from the last sync what a proposed action would take down and whether it risks your way back in. It never reports safe.

  _Use before any change that could cut the path needed to undo it. Verdicts are blocks-path, at-risk, no-known-risk or cannot-determine, and it cannot see every cause of path loss._

  ```bash
  unifi-pp-cli impact device restart gateway --agent
  ```
- **`change apply`** — Send one change to an official Network or Protect path with a journaled before-image, a read-back check and undo. Nothing is sent without --confirm.

  _Use instead of a plain write when the change must be checked and revertible. Docs say cloud keys are read-only today, other cloud writes are unproven beyond one voucher create and delete, and --trial expiry is unverified._

  ```bash
  unifi-pp-cli change apply firewall-policy.json --trial --site 11111111-2222-3333-4444-555555555555 --agent
  ```

### Local history
- **`drift`** — See field-level config changes between two synced snapshots, marked attributed (in the change journal) or out-of-band.

  _Use to answer what changed in config and whether it went through change apply. Needs at least two syncs._

  ```bash
  unifi-pp-cli drift --agent
  ```
- **`digest`** — One timeline, built from local syncs, of state changes, ISP dips, drift, newcomers and journaled changes.

  _Use for a catch-up summary instead of querying each product. A flap between two syncs is missed._

  ```bash
  unifi-pp-cli digest --since 7d --agent
  ```
- **`newcomer`** — List clients, devices and cameras first seen at a sync in a window.

  _Use to spot unfamiliar hardware. Randomized MACs can inflate the list, and a visit between syncs is missed._

  ```bash
  unifi-pp-cli newcomer --since 24h --agent
  ```

### Understand your network
- **`rule-predict`** — Predict which firewall policy would match a hypothetical packet. This is a local prediction with no API confirmation.

  _Use before editing rules. Treat indeterminate as unknown; legacy firewall rules are outside the model._

  ```bash
  unifi-pp-cli rule-predict --src-zone LAN --dst-zone External --dst-port 443 --protocol tcp --agent
  ```
- **`topology`** — See gateway, switches and APs as a tree per console, from the last sync.

  _Use to orient on the physical layout. Uplinks come from per-device detail requests, so sync --no-hydrate leaves the tree flat._

  ```bash
  unifi-pp-cli topology --agent
  ```

## Command Reference

**carrier**: Manage carrier

- `unifi-pp-cli carrier assign-subscriber-plan`: Assigns the subscriber's service plan.
- `unifi-pp-cli carrier attach-subscriber-host`: Attaches (or re-links) the subscriber's gateway host.
- `unifi-pp-cli carrier create-subscriber`: Creates a subscriber (identity, optional service address, and optionally a plan; attach a gateway host separately).
- `unifi-pp-cli carrier detach-subscriber-host`: Detaches the subscriber's gateway host. **Not idempotent.
- `unifi-pp-cli carrier get-service-plan`: Requires scope `read:plans`. Returns 400 if `id` is not a valid UUID.
- `unifi-pp-cli carrier get-subscriber`: Requires scope `read:subscribers`.
- `unifi-pp-cli carrier list-service-plans`: Lists the org's service plans (read-only). Requires scope `read:plans`.
- `unifi-pp-cli carrier list-subscribers`: Lists subscribers visible to the key (in-scope subscribers only). Cursor-paginated. Requires scope `read:subscribers`.
- `unifi-pp-cli carrier resume-subscriber`: Resumes the subscriber's service. Idempotent: resuming a not-suspended subscriber is a 200 no-op.
- `unifi-pp-cli carrier suspend-subscriber`: Suspends the subscriber's service (subscriber-level).
- `unifi-pp-cli carrier update-subscriber`: Updates name / email / notes / service address / plan / subscriber number / metadata.

**devices**: Manage devices

- `unifi-pp-cli devices`: Retrieves a list of UniFi devices managed by hosts where the UI account making the API call is the owner or a super

**hosts**: Manage hosts

- `unifi-pp-cli hosts get-by-id`: Retrieves detailed information about a specific host by ID.
- `unifi-pp-cli hosts list`: Retrieves a list of all hosts associated with the UI account making the API call.

**innerspace**: Manage innerspace

- `unifi-pp-cli innerspace get-asset`: Serves a floor plan asset file for the given plan ID and filename.
- `unifi-pp-cli innerspace list-access-points`: Returns placed UniFi access points with position, orientation, mounting details, and status.
- `unifi-pp-cli innerspace list-floor-plans`: Returns floor plans for RF planning import including image URLs, scale (pixels per meter), and coordinate origin.
- `unifi-pp-cli innerspace list-inventory`: Returns managed access points and switches that are not placed on any floor plan. Optionally filter by `siteId`.
- `unifi-pp-cli innerspace list-project`: Returns the project state for external integrations including shapes, plans, products, wall types
- `unifi-pp-cli innerspace list-switches`: Returns placed UniFi switches with position and status. Optionally filter by `siteId`.

**isp-metrics**: Manage isp metrics

- `unifi-pp-cli isp-metrics <type>`: Retrieves ISP metrics data for all sites linked to the UI account's API key.

**mobility**: Manage mobility

- `unifi-pp-cli mobility get-device`: Returns full detail for a single device, including WAN, cellular, WiFi, VPN, firewall rules, subscription
- `unifi-pp-cli mobility list-device-clients`: Returns a paginated list of all clients associated with a device, including ONLINE, OFFLINE, and BLOCKED clients.
- `unifi-pp-cli mobility list-devices`: Returns a paginated list of devices in the workspace. Each item contains identity and status fields only.
- `unifi-pp-cli mobility list-workspace-admins`: Returns all admins for a workspace with their permission levels. The caller must have active membership.
- `unifi-pp-cli mobility list-workspaces`: Returns all workspaces visible to the authenticated user. Only workspaces with active membership are returned.
- `unifi-pp-cli mobility update-device-name`: Updates the user-assigned display name of a device.
- `unifi-pp-cli mobility update-device-network`: Partial update: only provided fields are modified. WAN, IPv6, and InternetSource are not configurable.
- `unifi-pp-cli mobility update-device-wireless`: Replaces the WiFi SSID and password. Both fields are required.

**network**: Endpoints for creating, updating, deleting, and inspecting network configurations
including VLANs, DHCP, NAT, and IPv4/IPv6 settings.

- `unifi-pp-cli network adopt-device`: Adopt a device to a site.
- `unifi-pp-cli network create`: Create a new network on a site.
- `unifi-pp-cli network create-acl-rule`: Create a new user defined ACL rule on a site.
- `unifi-pp-cli network create-dns-policy`: Create a new DNS policy on a site.
- `unifi-pp-cli network create-firewall-policy`: Create a new firewall policy on a site.
- `unifi-pp-cli network create-firewall-zone`: Create a new custom firewall zone on a site.
- `unifi-pp-cli network create-hotspot-voucher`: Create one or more Hotspot vouchers.
- `unifi-pp-cli network create-traffic-matching-list`: Create a new traffic matching list on a site.
- `unifi-pp-cli network create-wifi-broadcast`: Create a new Wifi Broadcast on the specified site.
- `unifi-pp-cli network delete`: Delete an existing network on a site.
- `unifi-pp-cli network delete-acl-rule`: Delete an existing user defined ACL rule on a site.
- `unifi-pp-cli network delete-dns-policy`: Delete an existing DNS policy on a site.
- `unifi-pp-cli network delete-firewall-policy`: Delete an existing firewall policy on a site.
- `unifi-pp-cli network delete-firewall-zone`: Delete a custom firewall zone from a site.
- `unifi-pp-cli network delete-hotspot-vouchers`: Remove Hotspot vouchers based on the specified filter criteria.
- `unifi-pp-cli network delete-hotspot-voucher`: Remove a specific Hotspot voucher.
- `unifi-pp-cli network delete-traffic-matching-list`: Delete an exist traffic matching list on a site.
- `unifi-pp-cli network delete-wifi-broadcast`: Delete an existing Wifi Broadcast from the specified site.
- `unifi-pp-cli network get`: Retrieve detailed information about a specific network.
- `unifi-pp-cli network get-acl-rule`: Get ACL Rule
- `unifi-pp-cli network get-acl-rule-ordering`: Retrieve user-defined ACL rule ordering on a site.
- `unifi-pp-cli network get-client`: Retrieve detailed information about a specific connected client, including name, IP address, MAC address
- `unifi-pp-cli network get-device`: Retrieve detailed information about a specific adopted device, including firmware versioning, uplink state
- `unifi-pp-cli network get-device-statistics`: Retrieve the latest real-time statistics of a specific adopted device, such as uptime, data transmission rates
- `unifi-pp-cli network get-dns-policy`: Retrieve specific DNS policy.
- `unifi-pp-cli network get-firewall-policy`: Retrieve specific firewall policy.
- `unifi-pp-cli network get-firewall-policy-ordering`: Retrieve user-defined firewall policy ordering for a specific source/destination zone pair.
- `unifi-pp-cli network get-firewall-zone`: Get a firewall zone on a site.
- `unifi-pp-cli network get-hotspot-voucher`: Retrieve details of a specific Hotspot voucher.
- `unifi-pp-cli network get-references`: Retrieve references to a specific network.
- `unifi-pp-cli network get-switching-lag`: Retrieve LAG details.
- `unifi-pp-cli network get-switching-mc-lag-domain`: Retrieve MC-LAG Domain details.
- `unifi-pp-cli network get-switching-switch-stack`: Retrieve Switch Stack details.
- `unifi-pp-cli network get-traffic-matching-list`: Get an exist traffic matching list on a site.
- `unifi-pp-cli network get-wifi-broadcast`: Retrieve detailed information about a specific Wifi.
- `unifi-pp-cli network get-info`: Retrieve general information about the UniFi Network application.
- `unifi-pp-cli network list`: Retrieve a paginated list of all Networks on a site.
- `unifi-pp-cli network list-acl-rules`: Retrieve a paginated list of all ACL rules on a site.
- `unifi-pp-cli network list-clients`: Retrieve a paginated list of all connected clients on a site, including physical devices (computers, smartphones)
- `unifi-pp-cli network list-countries`: Returns ISO-standard country codes and names, used for region-based configuration or regulatory compliance.
- `unifi-pp-cli network list-device-tags`: Returns all device tags defined within a site, which can be used for WiFi Broadcast assignments.
- `unifi-pp-cli network list-devices`: Retrieve a paginated list of all adopted devices on a site, including basic device information.
- `unifi-pp-cli network list-dns-policies`: Retrieve a paginated list of all DNS policies on a site.
- `unifi-pp-cli network list-dpi-applications`: Lists DPI-recognized applications grouped under categories. Useful for firewall or traffic analytics integration.
- `unifi-pp-cli network list-dpi-categories`: Returns predefined Deep Packet Inspection (DPI) application categories used for traffic identification and filtering.
- `unifi-pp-cli network list-firewall-policies`: Retrieve a list of all firewall policies on a site.
- `unifi-pp-cli network list-firewall-zones`: Retrieve a list of all firewall zones on a site.
- `unifi-pp-cli network list-hotspot-vouchers`: Retrieve a paginated list of Hotspot vouchers.
- `unifi-pp-cli network list-pending-devices`: Retrieve a paginated list of devices pending adoption, including basic device information.
- `unifi-pp-cli network list-radius-profiles`: Returns available RADIUS authentication profiles, including configuration origin metadata.
- `unifi-pp-cli network list-sites`: Retrieve a paginated list of local sites managed by this Network application.
- `unifi-pp-cli network list-switching-lags`: Retrieve a paginated list of all LAGs (Link Aggregation Groups) on a site.
- `unifi-pp-cli network list-switching-mc-lag-domains`: Retrieve a paginated list of all MC-LAG Domains on a site.
- `unifi-pp-cli network list-switching-switch-stacks`: Retrieve a paginated list of all Switch Stacks on a site.
- `unifi-pp-cli network list-traffic-matching-lists`: Retrieve all traffic matching lists on a site.
- `unifi-pp-cli network list-vpn-servers`: Retrieve a paginated list of all VPN servers on a site.
- `unifi-pp-cli network list-vpn-site-to-site-tunnels`: Retrieve a paginated list of all site-to-site VPN tunnels on a site.
- `unifi-pp-cli network list-wans`: Returns available WAN interface definitions for a given site, including identifiers and names.
- `unifi-pp-cli network list-wifi-broadcasts`: Retrieve a paginated list of all Wifi Broadcasts on a site.
- `unifi-pp-cli network patch-firewall-policy`: Patch an existing firewall policy on a site.
- `unifi-pp-cli network remove-device`: Removes (unadopts) an adopted device from the site. If the device is online, it will be reset to factory defaults.
- `unifi-pp-cli network run-client-action`: Perform an action on a specific connected client.
- `unifi-pp-cli network run-device-action`: Perform an action on an specific adopted device.
- `unifi-pp-cli network run-port-action`: Perform an action on a specific device port.
- `unifi-pp-cli network set-acl-rule-ordering`: Reorder user-defined ACL rules on a site.
- `unifi-pp-cli network set-firewall-policy-ordering`: Reorder user-defined firewall policies for a specific source/destination zone pair.
- `unifi-pp-cli network update`: Update an existing network on a site.
- `unifi-pp-cli network update-acl-rule`: Update an existing user defined ACL rule on a site.
- `unifi-pp-cli network update-dns-policy`: Update an existing DNS policy on a site.
- `unifi-pp-cli network update-firewall-policy`: Update an existing firewall policy on a site.
- `unifi-pp-cli network update-firewall-zone`: Update a firewall zone on a site.
- `unifi-pp-cli network update-traffic-matching-list`: Update an exist traffic matching list on a site.
- `unifi-pp-cli network update-wifi-broadcast`: Update an existing Wifi Broadcast on the specified site.

**protect**: Manage protect

- `unifi-pp-cli protect alarm-hub-trigger`: Trigger an alarm hub output channel.
- `unifi-pp-cli protect alarm-webhook`: Send a webhook to the alarm manager to trigger configured alarms
- `unifi-pp-cli protect arm-disable`: Disable the arm alarm feature. Only available when using local alarm manager.
- `unifi-pp-cli protect arm-enable`: Enable the arm alarm feature using the currently selected arm profile. Only available when using local alarm manager.
- `unifi-pp-cli protect create-arm-profile`: Create a new arm profile. Only available when using local alarm manager.
- `unifi-pp-cli protect create-camera-rtsps-stream`: Returns RTSPS stream URLs for specified quality levels
- `unifi-pp-cli protect create-file`: Upload a new device asset file
- `unifi-pp-cli protect create-liveview`: Create a new live view
- `unifi-pp-cli protect create-talkback-session`: Returns the talkback stream URL and audio configuration for a specific camera
- `unifi-pp-cli protect delete-arm-profile`: Delete an arm profile by ID. Only available when using local alarm manager.
- `unifi-pp-cli protect delete-camera-rtsps-stream`: Remove the RTSPS stream for a specified camera
- `unifi-pp-cli protect disable-camera-mic-permanently`: Disable the microphone for a specific camera. This action cannot be undone unless the camera is reset.
- `unifi-pp-cli protect get-alarm-hub`: Get detailed information about a specific alarm hub
- `unifi-pp-cli protect get-bridge`: Get detailed information about a specific bridge
- `unifi-pp-cli protect get-camera`: Get detailed information about a specific camera
- `unifi-pp-cli protect get-camera-rtsps-stream`: Returns existing RTSPS stream URLs for camera
- `unifi-pp-cli protect get-camera-snapshot`: Get a snapshot image from a specific camera.
- `unifi-pp-cli protect get-chime`: Get detailed information about a specific chime
- `unifi-pp-cli protect get-file`: Get a list of all device asset files
- `unifi-pp-cli protect get-fob`: Get detailed information about a specific fob
- `unifi-pp-cli protect get-light`: Get detailed information about a specific light
- `unifi-pp-cli protect get-link-station`: Get detailed information about a specific link station (non-alarm hub gateways)
- `unifi-pp-cli protect get-liveview`: Get detailed information about a specific live view
- `unifi-pp-cli protect get-relay`: Get detailed information about a specific relay
- `unifi-pp-cli protect get-sensor`: Get detailed information about a specific sensor
- `unifi-pp-cli protect get-siren`: Get detailed information about a specific siren
- `unifi-pp-cli protect get-speaker`: Get detailed information about a specific speaker
- `unifi-pp-cli protect get-ulp-user`: Get detailed information about a specific UniFi Identity user.
- `unifi-pp-cli protect get-user`: Get detailed information about a specific Protect user.
- `unifi-pp-cli protect get-viewer`: Get detailed information about a specific viewer
- `unifi-pp-cli protect get-info`: Get generic information about the Protect application
- `unifi-pp-cli protect list-alarm-hubs`: Get detailed information about all alarm hubs
- `unifi-pp-cli protect list-arm-profiles`: Get a list of all arm profiles. Only available when using local alarm manager.
- `unifi-pp-cli protect list-bridges`: Get detailed information about all bridges
- `unifi-pp-cli protect list-cameras`: Get detailed information about all cameras
- `unifi-pp-cli protect list-chimes`: Get detailed information about all chimes
- `unifi-pp-cli protect list-fobs`: Get detailed information about all fobs
- `unifi-pp-cli protect list-lights`: Get detailed information about all lights
- `unifi-pp-cli protect list-link-stations`: Get detailed information about all link stations (non-alarm hub gateways)
- `unifi-pp-cli protect list-liveviews`: Get detailed information about all live views
- `unifi-pp-cli protect list-nvrs`: Get detailed information about the NVR
- `unifi-pp-cli protect list-relays`: Get detailed information about all relays
- `unifi-pp-cli protect list-sensors`: Get detailed information about all sensors
- `unifi-pp-cli protect list-sirens`: Get detailed information about all sirens
- `unifi-pp-cli protect list-speakers`: Get detailed information about all speakers
- `unifi-pp-cli protect list-ulp-users`: Get all UniFi Identity users with enrolled credentials (NFC cards, fingerprints).
- `unifi-pp-cli protect list-users`: Get all Protect users. Users are filtered based on access permissions.
- `unifi-pp-cli protect list-viewers`: Get detailed information about all viewers
- `unifi-pp-cli protect patch-alarm-hub`: Patch the settings for a specific alarm hub
- `unifi-pp-cli protect patch-arm-profile`: Update an existing arm profile. Only available when using local alarm manager.
- `unifi-pp-cli protect patch-bridge`: Patch the settings for a specific bridge
- `unifi-pp-cli protect patch-camera`: Patch the settings for a specific camera
- `unifi-pp-cli protect patch-chime`: Patch the settings for a specific chime
- `unifi-pp-cli protect patch-fob`: Patch the settings for a specific fob
- `unifi-pp-cli protect patch-light`: Patch the settings for a specific light
- `unifi-pp-cli protect patch-link-station`: Patch the settings for a specific link station
- `unifi-pp-cli protect patch-liveview`: Patch the configuration about a specific live view
- `unifi-pp-cli protect patch-relay`: Patch the settings for a specific relay
- `unifi-pp-cli protect patch-sensor`: Patch the settings for a specific sensor
- `unifi-pp-cli protect patch-siren`: Patch the settings for a specific siren
- `unifi-pp-cli protect patch-speaker`: Patch the settings for a specific speaker
- `unifi-pp-cli protect patch-viewer`: Patch the settings for a specific viewer
- `unifi-pp-cli protect pos-transaction`: Records a point of sale transaction as a camera event so the transaction details can be overlaid on recorded footage.
- `unifi-pp-cli protect ptz-goto`: Adjust the PTZ camera position to a specified preset
- `unifi-pp-cli protect ptz-patrol-start`: Start a camera PTZ patrol
- `unifi-pp-cli protect ptz-patrol-stop`: Stop active camera PTZ patrol
- `unifi-pp-cli protect relay-activate`: Control a relay output state. Use 'state' to set a specific state (on/off), or omit to toggle.
- `unifi-pp-cli protect set-arm-settings`: Set the current arm profile to be used when enabling the arm alarm. Only available when using local alarm manager.
- `unifi-pp-cli protect siren-play`: Activate the siren alarm for the specified duration.
- `unifi-pp-cli protect siren-stop`: Stop an active siren.
- `unifi-pp-cli protect siren-test-sound`: Test the siren sound for 5 seconds at the specified volume.
- `unifi-pp-cli protect speaker-test-sound`: Test the speaker sound at the specified volume.

**sd-wan-configs**: Manage sd wan configs

- `unifi-pp-cli sd-wan-configs get-sdwan-config-by-id`: Retrieves detailed information about a specific SD-WAN configuration by ID.
- `unifi-pp-cli sd-wan-configs list-sdwan-configs`: Retrieves a list of all SD-WAN configurations associated with the UI account making the API call.

**sites**: Manage sites

- `unifi-pp-cli sites`: Retrieves a list of all sites (from hosts running the UniFi Network application)


**raw**: console paths under `/proxy`, including unofficial and unverified controller paths

- `unifi-pp-cli raw`: Reads run directly; other methods preview until `--confirm`. Site Manager paths are refused.

**fleet**: every console the key can see (needs `UNIFI_API_KEY`)

- `unifi-pp-cli fleet`: Health, inventory and firmware. Partial failures exit 0, so read `fetch_failures` and `connector` per console.

**audit**: local log of attempted writes

- `unifi-pp-cli audit`: Outcomes are `applied`, `refused` and `ambiguous`. An `ambiguous` write may have applied, so read the object before trying again.

**sync**: local mirror and snapshots

- `unifi-pp-cli sync`: Mirrors the selected console and takes a snapshot. `--max-hydrate` caps device detail requests (default 100, `0` for no cap). `--no-hydrate` skips them, leaving `topology` flat and `impact` less able to say what sits below a device.

**change**


- `unifi-pp-cli change apply`: Send one change to an official Network or Protect path with a journaled before-image, a read-back check and undo. Nothing is sent without --confirm.
- `unifi-pp-cli change list`: List journaled changes, so an undo can find its id.
- `unifi-pp-cli change undo`: Revert a journaled change when current state still matches its after-image. Nothing is sent without `--confirm`.


**digest**


- `unifi-pp-cli digest`: One timeline, built from local syncs, of state changes, ISP dips, drift, newcomers and journaled changes.


**drift**


- `unifi-pp-cli drift`: See field-level config changes between two synced snapshots, marked attributed (in the change journal) or out-of-band.


**impact**


- `unifi-pp-cli impact`: Estimate from the last sync what a proposed action would take down and whether it risks your way back in. It never reports safe.


**newcomer**


- `unifi-pp-cli newcomer`: List clients, devices and cameras first seen at a sync in a window.


**reach**


- `unifi-pp-cli reach`: See which transports and products answer on each console. Write status is yes, no or unproven, and a probe alone never reports yes.


**rule-predict**


- `unifi-pp-cli rule-predict`: Predict which firewall policy would match a hypothetical packet. This is a local prediction with no API confirmation. This is a local prediction with no API confirmation.


**topology**


- `unifi-pp-cli topology`: See gateway, switches and APs as a tree per console, from the last sync.




### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
unifi-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match: fall back to `--help` or use a narrower query. `--json` (and other machine formats) keep that exit-2 contract and write `{"matches":[]}` on stdout so agents can inspect the envelope without treating a miss as success.

## Recipes

### Check before changing

```bash
unifi-pp-cli impact device restart gateway --agent
```

Reports affected clients and a control-path verdict from the last sync. Run sync first, or the verdict is cannot-determine.

### Narrow device output

```bash
unifi-pp-cli network list-devices 11111111-2222-3333-4444-555555555555 --agent --select data.name,data.model,data.state
```

Keeps large device payloads small for agents. Get the site id from network list-sites.

### Attribute changes

```bash
unifi-pp-cli drift --agent
```

Marks each change attributed when the change journal holds it, otherwise out-of-band. Writes made outside change apply count as out-of-band.

### Test a local route

```bash
unifi-pp-cli reach --gateway 192.168.1.1 --json
```

Probes the gateway with UNIFI_LOCAL_API_KEY. Works without a cloud key.

### Preview a journaled change

```bash
unifi-pp-cli change apply --change-json '{"method":"PATCH","path":"/network/integration/v1/sites/SITE/firewall/policies/POLICY","body":{"enabled":false}}' --json
```

Prints the plan and sends nothing. Add --confirm to send it.

## Auth Setup

Cloud: create a key at unifi.ui.com under Settings > API Keys and set UNIFI_API_KEY. Local: create a key in the Network app under Settings > Control Plane > Integrations, set UNIFI_LOCAL_API_KEY, and pass --gateway or set UNIFI_GATEWAY. The keys are separate, and a cloud key is never sent to a gateway. Run 'reach' to see what answers.

Run `unifi-pp-cli doctor` to verify setup.

## Choosing a Transport

Network, Protect and InnerSpace commands run on a console and need a route:

- **Cloud connector:** `--console` or `UNIFI_CONSOLE_ID`, with `UNIFI_API_KEY`. Use it off-site, with several consoles, or for reads. Requires console firmware 5.0.3 or newer per the spec. Documented limits are 100 requests per minute per console, 25 seconds per request and 10 MB per response.
- **Local gateway:** `--gateway` or `UNIFI_GATEWAY`, with `UNIFI_LOCAL_API_KEY`. Use it to write, or when the machine can reach the gateway.

Rules:

- Flags beat env, and env beats config (`console_id`, `gateway`, `local_api_key`, `transport`).
- A gateway is used whenever one is set and `--transport` is not `cloud`. `--transport local` with no gateway fails with exit 10.
- With neither set, one visible console is used automatically. Several are an error (exit 10) that lists the ids.
- `hosts`, `devices`, `sites`, `isp-metrics`, `sd-wan-configs`, `carrier`, `mobility` and `fleet` always call `api.ui.com` with `UNIFI_API_KEY`, even with `--gateway` set.
- `UNIFI_INSECURE_SKIP_VERIFY` overrides gateway certificate checks. By default they are skipped for literal private addresses only.

Controls: `UNIFI_READ_ONLY` (any value except empty, `0`, `false`, `no` or `off`) refuses all API writes, with exit 10. `UNIFI_CONSOLE_ALLOWLIST` (comma-separated ids) limits connector requests. Set `UNIFI_CONSOLE_ID`. MCP callers then cannot target another console unless it is listed in `UNIFI_CONSOLE_ALLOWLIST`. Site Manager reads and writes stay account-wide. A named console is also checked on the local route, and local-only commands refuse a `--console` outside the list (exit 10). A configured gateway with no console named stays outside the allowlist, and so do Site Manager commands and rows already in the mirror. Over MCP, `change apply` takes `--change-json`, because file paths are refused.

Writes are not replayed after a timeout, 408 or 5xx, because they may have applied. Read state and check `audit` before trying again. `unifi-pp-mcp` has no `--console` or `--gateway` flag, its `--transport` means `stdio` or `http`, and its route comes from the environment.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color`.

Global format flags share one contract on promoted, novel, sync, and `--deliver` paths:

- `--json`: one JSON document on stdout (sync progress events go to stderr)
- `--compact`: keep identity/status/timestamp fields; does not change the document vs stream shape
- `--csv` / `--plain`: tabular rows (collection envelopes unwrap to the row array). `--csv` prefixes `'` on a cell or header key that starts with `=`, `+`, `-`, `@`, tab, CR, or LF (fullwidth `＝＋－＠` too) so a spreadsheet does not run it as a formula; a plain signed number such as `-5` or `+2.5e3` is unchanged.
- `--quiet`: one identity value per row, no envelope

- **Pipeable**: JSON on stdout, errors on stderr
- **Filterable**: `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  unifi-pp-cli devices --agent --select devices,hostId,hostName
  ```
- **Previewable**: `--dry-run` shows the request without sending. A completed dry run exits 0 and its output includes `"dry_run": true`, but the rest of the shape varies by command, and `"success"` is not always present. Validation failures exit non-zero.
- **Offline-friendly**: sync/search commands can use the local SQLite store when available
- **Non-interactive**: never prompts, every input is a flag
- **Confirmation varies**: `network`, `protect`, `carrier` and `mobility` writes send immediately. `raw`, `change apply` and `change undo` need `--confirm`. `--yes` applies only to local commands such as `local-data purge` and `profile delete`. `--agent` implies none of these
- **Explicit retries**: use `--idempotent` only when an already-existing create should count as success, and use `--ignore-missing` only when a missing delete target should count as success

### Response envelope

With `--agent`, most commands that read from the local store or the API wrap output in a provenance envelope. Some hand-written commands print their own shape, for example `raw`, `audit --json` and `change list --json`:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set: piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `UNIFI_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `UNIFI_CONFIG_DIR`, `UNIFI_DATA_DIR`, `UNIFI_STATE_DIR`, `UNIFI_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `UNIFI_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `unifi-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "unifi": {
        "command": "unifi-pp-mcp",
        "env": {
          "UNIFI_HOME": "/srv/unifi"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `UNIFI_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `UNIFI_HOME`, or `doctor` will not find credentials left under the former root.

Local data is unencrypted and relies on OS account permissions plus disk encryption. Reads in the default `--data-source auto` mode are written to the SQLite store; GET responses are cached for five minutes under the cache dir. `local-data purge` previews what it would erase (store files, response cache, learn journal) and deletes only with `--yes`; it is not exposed over MCP. `UNIFI_NO_LOCAL_DATA=1` disables the response cache and store writes, turns `auto` reads into live reads, and blocks `sync` and `workflow archive`; `--data-source local` still reads an existing store, and `UNIFI_DATA_SOURCE` sets the `--data-source` default. Secret fields are stored only as salted digests. The salt is in the state directory, not with the database. GET responses holding a secret are not cached.

## Automatic learning

This CLI ships a self-capturing learning loop. The CLI does its own bookkeeping: every invocation is journaled locally, a failed flag followed by a corrected retry auto-derives a `flag_alias` candidate, and a `teach` on a query family without a playbook auto-synthesizes a `playbook_candidate` from the session's journal. Your job is judgment only: `recall` first, act on surfaced candidates, `teach` the final answer, `playbook amend` when you observe a correction. You never record failures by hand.

### Step 1: `recall` before any discovery

Before list/search/drill commands on a new user question, pass the question as an argv or MCP tool argument to `recall --agent`. Do not interpolate user-controlled text into a shell command line.

Quoted `recall "<question>"` breaks on an apostrophe, which is ordinary English. A quoted heredoc breaks when a body line equals the delimiter, and that delimiter is published in these docs. Write the question with a non-shell file-writing tool, then read it back as data:

```bash
# Write the question verbatim with your file-writing tool (no shell involved).
# Command substitution on a file only ever yields data: the shell never
# parses the file's bytes as syntax.
QUERY=$(cat /path/to/question.txt)
unifi-pp-cli recall "$QUERY" --agent
```

Prefer MCP: pass the question as the tool's query argument. `"$QUERY"` after a file read is argv-safe; putting the question itself in the command text is not.

The response envelope:

```json
{
  "query": "...",
  "normalized": "<normalized form>",
  "query_entities": ["..."],
  "found": true | false,
  "match_score": 0.0,
  "results": [
    { "resource_id": "...", "resource_type": "...", "venue": "...",
      "confidence": 2, "entity_match": "exact|partial|unknown",
      "source": "taught|preseed|pattern", "warnings": ["..."] }
  ],
  "mismatches": [ /* only when --debug-mismatches */ ],
  "warnings": [ /* top-level */ ],
  "candidates": [
    { "id": 12, "class": "flag_alias | playbook_candidate",
      "summary": "...", "sightings": 3, "last_seen": "...",
      "rationale": "...",
      "next_action": ["<trial command>", "unifi-pp-cli learnings confirm 12"] }
  ],
  "playbook": {
    "query_family": "...",
    "playbook": {
      "steps": [ { "cmd": "<command with {slot} substitution>", "purpose": "..." } ],
      "entity_slots": ["$ENTITY"],
      "expected_tool_calls": 3
    },
    "slots_resolved": { "$ENTITY": { "token": "<live token>", "canonical": "<canonical>" } },
    "notes": "<workarounds + gotchas for this query family>"
  },
  "notes": "<duplicate surface for non-playbook callers>"
}
```

Empty-store short-circuit: if the store has no learnings, playbooks, or candidates yet (recall finds nothing and `learnings list` and `learnings candidates` are both empty), skip recall for the rest of this session instead of taxing every query; resume recall-first once something has been taught.

### Step 2: decision tree

Read `candidates`, `playbook`, `notes`, `results[0]`, and warnings in that order:

```
if Candidates present (warnings include "candidates_present"):
    -> candidates are try-then-confirm, never facts. Follow each candidate's
       two-step next_action verbatim: run the trial command first, then run
       `learnings confirm <id>` only after the trial verified the behavior.
       Reject a wrong candidate with `learnings reject <id>`.
    -> NEVER re-teach something recall surfaced as a candidate; confirm or
       reject that candidate instead of teaching a duplicate.
    -> candidates ride alongside playbooks and resource hits, not instead of
       them; continue with the branches below after acting on them.

if Playbook present:
    -> READ Playbook.notes verbatim FIRST (workarounds + gotchas the CLI surface doesn't expose)
    -> replay Playbook.steps in order, substituting Playbook.slots_resolved entries
       for the entity slot tokens. If a step's slot is unresolved, fall back to
       discovery for that step only.
    -> the Playbook's expected_tool_calls is a budget; if you find yourself running
       materially more, record the divergence via `unifi-pp-cli playbook amend`
       at end-of-session.

elif Notes present (no Playbook):
    -> read Notes verbatim before any discovery step; they carry known gotchas
       for this query family even when no structured choreography exists yet.

elif Found AND Results[0].EntityMatch == "exact" AND Results[0].Confidence >= 2:
    -> skip discovery; fetch live data for Results[*].ResourceID in parallel

elif Found AND Results[0].EntityMatch == "partial":
    -> candidate hint, NOT a hit; read the resource title to validate before trusting

elif (any row in Mismatches[] when --debug-mismatches was passed):
    -> treat as cold start; the stored learning is for a different entity
       (different canonical resolved from query_entities)

else:  // Found == false, no playbook, no notes
    -> cold start; run discovery normally; teach the answer afterward (Step 4).
       If the family has no playbook yet, that teach auto-synthesizes a
       playbook candidate from this session's journal - you do not need to
       record one by hand.
```

Playbook and Notes are orthogonal to the per-resource path. A recall response can carry both a Playbook AND a `Results[]` hit - use both: the Playbook tells you which choreography to run; the resource hits short-circuit specific steps. Default to skipping `mismatches`; pass `--debug-mismatches` only when investigating cold-start surprises.

Candidate judgment details: `learnings confirm <id>` prints the candidate's full payload before materializing it - check that the printed payload matches the behavior you verified. `learnings reject <id>` tombstones the derivation signature so the same candidate does not resurface. The envelope carries only the few candidates worth acting on now; `unifi-pp-cli learnings candidates` lists the full open set.

Graceful degradation: if `learnings confirm` is an unknown command, you are driving an older binary - ignore the candidates guidance and follow the rest of the protocol.

### Step 3: always read `warnings`

- `low_confidence`: row exists at `confidence<2`. Treat as a hint, not a skip-discovery hit.
- `resource_not_in_store`: the local store doesn't have the resource the learning points at. The match validator couldn't classify entities: direct-fetch and re-evaluate.
- `cross_alias_match` (per-result): the row was taught under a different alias and matched the live query's canonical via `entity_lookups` (e.g., a "main office" teach satisfying an "HQ" recall). Trust the resource_id.
- `similar_shape_different_entity:<canonical>` (top-level): a structurally matching row exists but its canonical entity differs from the live query's. Treated as cold start; the warning carries the conflicting canonical as a hint, but the row is NOT promoted into Results.
- `ambiguous_alias` (top-level): a single query entity resolved to multiple canonicals (e.g., "Office" → two sites with that name). Surface the ambiguity from context before committing to a resource.
- `candidates_present` (top-level): the envelope carries a `candidates` section. Handle it via the candidates branch in Step 2 before anything else.
- `lookup_refresh_available` (top-level): an entity in the query has no lookup row yet, but synced data could provide one. Run `unifi-pp-cli sync` to refresh entity lookups.
- Top-level `no_learnings_for_query_family`: the table had no rows above the Jaccard floor. Pure cold start.

### Step 4: `teach &` after finalizing your response - always

Teaching is unconditional. After resolving a query the store could not answer, background-teach the final resource mapping - no call-count threshold, no judging whether it was "worth" learning. The teach is the anchor of the loop: it triggers playbook synthesis for a family without a playbook, and same-referent phrasings fold into one family so near-duplicate teaches do not fragment the store. Fire it after assembling your user-facing response but BEFORE emitting it, with a shell `&` so the call returns immediately. Pass the query the same way as recall: argv/MCP, or file-then-`$QUERY`. Do not splice the question into the command text:

```bash
QUERY=$(cat /path/to/question.txt)
unifi-pp-cli teach --query "$QUERY" --resource-type <type> --resource <id1> --resource <id2>
# (append shell `&` to background it)
```

Silent on success. Errors only land in `teach.log` under the resolved state dir. Teach the **most specific** resource - if the user asked a broad question and you walked through parent records to find the specific answer, teach the leaf id, not the parent. The CLI uses seeded `entity_lookups` for cross-alias resolution at recall time, so a teach under one alias (e.g., "main office") satisfies future queries under another alias (e.g., "HQ") automatically.

PII rule: teach the structural question with identifiers stripped - never include names, emails, phone numbers, account ids, or other personal identifiers in taught queries or notes. The CLI scans teach queries for obvious email/phone shapes and warns, but does not block; strip before teaching rather than relying on the warning.

### Step 5: playbooks - optional flags, automatic synthesis

You do not need to decide whether a session "deserves" a playbook: a teach on a family without one auto-synthesizes a `playbook_candidate` from the session's journal, and the next session judges it via confirm/reject. Attach explicit playbook flags only when you already hold choreography worth recording verbatim - workarounds the CLI didn't surface (silently-dropped flags, undocumented params, pagination tricks, payload gotchas). Prefer the **integrated one-call form** - record the resource learning and the playbook in the same `teach` invocation:

```bash
# Common case: record both the resource learning AND the playbook in one call.
QUERY=$(cat /path/to/question.txt)
unifi-pp-cli teach \
  --query "$QUERY" \
  --resource <id> \
  --playbook-file ~/playbooks/<shape>.json \
  --playbook-notes-file ~/playbooks/<shape>-notes.md
# (append shell `&` to background it)

# Alternate: playbook-only (no resource to record alongside).
QUERY=$(cat /path/to/question.txt)
unifi-pp-cli teach-playbook \
  --query "$QUERY" \
  --playbook-file ~/playbooks/<shape>.json \
  --notes-file ~/playbooks/<shape>-notes.md
```

Playbook files are JSON with `steps`, `entity_slots`, `expected_tool_calls`. Notes files are markdown carrying the gotchas verbatim. File-free callers (MCP-only agents) pass the same content inline: `--playbook-json` and `--playbook-notes` on the integrated `teach` form, `--playbook-json` and `--notes` on `teach-playbook`. On the integrated `teach` form, the playbook flags are optional - omit them entirely for a resource-only teach. On the standalone `teach-playbook` form, at least one of the playbook and notes flags must be set; both empty is rejected. Playbooks are keyed on the structural query family (entities stripped) so a recipe taught from one entity-shaped query applies to every other query of the same shape, with `slots_resolved` binding the live query's canonical at recall time.

When you DO find a playbook on a future recall, treat it as ground truth: replay the steps with `slots_resolved` substitutions, skip the discovery that the choreography already documents, and read `notes` before any step.

### Step 6: `playbook amend &` when your debug response identifies a correction

If your debug-protocol response identifies a concrete correction the notes or playbook should know: a workaround, an undocumented endpoint shape, a stale field name, observed schema drift, an empty-payload fallback: fire `playbook amend` BEFORE emitting your user-facing response. Same fire-and-forget posture as `teach`. Pass the query and note as argv/MCP arguments, or write each with a non-shell file tool and read them back (`QUERY=$(cat ...)`, `NOTE=$(cat ...)`). Do not interpolate either string into the command text:

```bash
QUERY=$(cat /path/to/question.txt)
NOTE=$(cat /path/to/note.txt)
unifi-pp-cli playbook amend \
  --query "$QUERY" \
  --add-note "$NOTE"
# (append shell `&` to background it)
```

What counts as worth amending: a behavior you OBSERVED this session that future-you would benefit from knowing. Examples worth amending:

- A workaround for a CLI surface that silently drops or misorders a flag.
- An undocumented endpoint shape (response wrapped in `{meta, results}`, payload nested two levels deeper than the docs claim).
- Observed schema drift (a field renamed, an index that shifted between firmware versions, a category label that the API now returns lower-cased).

What does NOT belong in notes:

- The entity-specific answer to the user's question. That's the response, not a learning.
- Per-site / per-device / per-row data the playbook already retrieves at runtime.
- Statements that paraphrase what the existing notes already say.

The amend command appends to the family's existing notes with a timestamped marker (`[amend YYYY-MM-DDTHH:MMZ]: <text>`). Multiple amends accumulate; the audit trail is visible. If no playbook exists yet for the family, amend creates a notes-only one (so cold-start corrections still land).

#### PII discipline for amend notes

`playbook amend` notes are designed to potentially flow upstream as shared knowledge in future versions of the Printing Press. Keep them clean of user-identifying content so the upstream-contribution path stays open without retroactive scrubbing:

- **Do NOT embed** paths to user filesystems, personal API keys or tokens, user email addresses, user GitHub handles, or specific query histories tied to a single user.
- **Acceptable**: endpoint shapes, undocumented field names, API gotchas, observed schema drift, workarounds for CLI surfaces, generalizable pagination or retry tactics.

If a correction is only meaningful with user-specific context, it belongs in a personal note, not in the playbook amend.

### Measuring the loop

`unifi-pp-cli learnings stats` reports recall hit rate, teach-to-reuse, playbook resolution rate, and candidate confirm/reject counts from the local `learn_events` table. Rates are null until they have a denominator; everything stays on this machine. Use it to check whether the loop is earning its keep for this CLI.

### Disabling learning

- `--no-learn` on a single command short-circuits both `recall` and the `teach` write path. Use for deterministic agent flows or tests that must not be affected by accumulated learnings.
- `UNIFI_NO_LEARN=true` in the environment globally disables the pipeline.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
unifi-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
unifi-pp-cli feedback --stdin < notes.txt
unifi-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `UNIFI_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `UNIFI_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Four sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` as a private 0600 file (exclusive randomly named temp in the destination directory, then rename; a planted symlink is not followed). Binary-response commands write decoded payload bytes (not the base64 JSON envelope) and print a small JSON receipt on stdout; `--json`/`--csv` do not refuse when this sink is set. |
| `webhook:<url>` | POST the output body to the URL (`application/json`). Redirects are not followed; a 3xx response fails the delivery and the body is not sent to the `Location`. |
| `spacefast:<name>` | Publish the output as a web page on the third-party Spacefast service and print the URL. Without `SPACEFAST_TOKEN` the page is published anonymously. Network and client data leaves this machine, so use it only when the user asks. |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures, including redirects, return non-zero and log the URL and HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled or recurring agent reuses the same saved flags while providing different input each run.

```
unifi-pp-cli profile save briefing --json
unifi-pp-cli --profile briefing devices
unifi-pp-cli profile list --json
unifi-pp-cli profile show briefing
unifi-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success (`fleet` and `reach` report per-console failures in their output and can still exit 0) |
| 1 | Other error, such as an unknown `--deliver` scheme |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required, including a missing `UNIFI_LOCAL_API_KEY` on the local route |
| 5 | API error, including a console's HTML page and an unresolved `change apply` |
| 6 | Partial failure |
| 7 | Rate limited (wait and retry) |
| 10 | Config error, including no console selected, a console outside the allowlist, and writes refused by read-only mode |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `unifi-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/devices/unifi/cmd/unifi-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add unifi-pp-mcp -- unifi-pp-mcp
   ```
3. Verify: `claude mcp list`

The MCP server takes its route and keys from its environment (`UNIFI_API_KEY`, `UNIFI_LOCAL_API_KEY`, `UNIFI_GATEWAY`, `UNIFI_CONSOLE_ID`, `UNIFI_TRANSPORT`). `unifi_execute`, `raw` and `change apply` preview until the caller passes `confirm: true`. That separates a preview from a send. It is not a human approval.

For a remote host, `unifi-pp-mcp --transport http` refuses to start unless `UNIFI_MCP_HTTP_TOKEN` has at least 32 characters (`openssl rand -hex 32` prints 64), and every request must send `Authorization: Bearer <token>`. It binds `127.0.0.1:7777` by default and refuses any non-loopback `--addr` without `--tls-cert` and `--tls-key`. A loopback `Origin` is accepted, and any other is refused unless listed in `UNIFI_MCP_HTTP_ALLOWED_ORIGINS` (comma-separated `scheme://host[:port]` values). Requests with no `Origin` header are accepted, so this stops browsers, not other clients.

Risk on a non-loopback address: there is one shared token with no per-caller identity, scopes or expiry, and no rate limit or lockout on failed attempts. The server is not read-only by default, so a token holder gets every tool with the authority of the server's keys, including writes. MCP callers cannot choose the route: `gateway` and `transport` are refused. MCP `sql` and `search` read the mirror, which stores secrets only as salted digests. `unifi_read` and `raw` still return what the live API sends. Prefer loopback with an SSH tunnel or VPN.
## Direct Use

1. Check if installed: `which unifi-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   unifi-pp-cli network list-sites --agent
   ```
4. If ambiguous, drill into subcommand help, for example `unifi-pp-cli network --help`.
5. Before any write, read "Before You Act" above. Do not add `--confirm` or send a write the user did not ask for.
