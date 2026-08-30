---
name: pp-3cx-xapi
description: "Every 3CX Configuration API (XAPI) object in one Go binary, with a local mirror of your whole PBX, offline search, config diffing, and cross-entity auditing no 3CX tool has. Trigger phrases: `audit my 3cx config`, `what changed on the pbx`, `provision 3cx extensions`, `queue sla report 3cx`, `trace extension 214`, `use 3cx-xapi`, `run 3cx-xapi`."
author: ""
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - 3cx-xapi-pp-cli
    install:
      - kind: go
        bins: [3cx-xapi-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/social-and-messaging/3cx-xapi/cmd/3cx-xapi-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/social-and-messaging/3cx-xapi/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# 3CX — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `3cx-xapi-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install 3cx-xapi --cli-only
   ```
2. Verify: `3cx-xapi-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/3cx-xapi/cmd/3cx-xapi-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Covers the full 3CX V20+ Configuration API: users, groups, ring groups, queues, IVRs, trunks, DIDs, in/outbound rules, reporting, backups, security, and integrations. Beyond CRUD, it mirrors the entire config graph into local SQLite so you can search it offline, diff it over time, audit it for dangling references, roll up queue and agent performance, and trace exactly what routes to an extension.

## When to Use This CLI

Use this CLI to manage and reason about a 3CX V20+ phone system from scripts or an agent: provisioning extensions in bulk, auditing the config graph for broken routing, diffing config over time, rolling up queue and agent performance, and tracing how calls reach an extension. It is ideal when you want offline, queryable, agent-native access to a PBX whose web console is slow and siloed.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI for real-time call control (answer, transfer, or originate a live call); that is the separate 3CX Call Control API, not the Configuration API.
- Do not use it to place or receive calls or as a softphone.
- Do not use it for end-user voicemail playback or webclient chat; it manages configuration and reporting, not the user-facing client.


## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Config graph intelligence
- **`audit`** — Flag every ring group, queue, inbound/outbound rule, and IVR destination that points at a deleted, disabled, or non-existent extension or DID.

  _Reach for this to catch broken call routing before users do, surfacing dangling references the 3CX console cannot detect._

  ```bash
  3cx-xapi-pp-cli audit --agent
  ```
- **`diff`** — Capture the full PBX config to a local snapshot, then show exactly what changed between two points in time or two tenants.

  _Reach for this for change management, proving what changed, when, and by how much across the whole config._

  ```bash
  3cx-xapi-pp-cli diff yesterday now --agent
  ```
- **`posture`** — Consolidate blocklist, blacklisted numbers, anti-hacking settings, firewall state, failed-auth events, and the admin audit log into one attack-surface report.

  _Reach for this for a one-shot security review instead of hunting across Security and Logs screens._

  ```bash
  3cx-xapi-pp-cli posture --agent
  ```
- **`trace`** — For one extension, show every inbound rule, ring group, queue, DID, and forwarding profile that routes to it.

  _Reach for this to answer why an extension is not ringing by seeing every path that reaches it._

  ```bash
  3cx-xapi-pp-cli trace 214 --agent
  ```

### Bulk operations
- **`provision`** — Idempotently create extensions and users from a CSV, assigning groups and roles, with a dry-run plan and typed exit codes.

  _Reach for this to onboard a whole tenant in one command instead of clicking the web console per extension._

  ```bash
  3cx-xapi-pp-cli provision --file users.csv --dry-run
  ```

### Contact-centre analytics
- **`qrollup`** — One cross-queue table joining queue performance, agent-in-queue stats, abandoned calls, and SLA breaches over a time window.

  _Reach for this to compare queues and weeks at a glance instead of opening one slow report screen per queue._

  ```bash
  3cx-xapi-pp-cli qrollup --since 7d --agent
  ```

### Local state that compounds
- **`search`** — Instant full-text search across the mirrored config graph (users, groups, ring groups, queues, trunks, DIDs, contacts) by number, name, email, or extension.

  _Reach for this when you know a number or name and need the object fast, across every entity type at once._

  ```bash
  3cx-xapi-pp-cli search reception --type users --data-source local --agent
  ```
- **`changed`** — A single time-windowed feed merging activity log, event log, active calls, and system status into the state of the PBX now or since a given time.

  _Reach for this as the first command when triaging something wrong right now on a PBX._

  ```bash
  3cx-xapi-pp-cli changed --since 2h --agent
  ```

## Command Reference

**active-calls** — Manage active calls

- `3cx-xapi-pp-cli active-calls` — Get entities from ActiveCalls

**active-calls-id** — Manage active calls id

- `3cx-xapi-pp-cli active-calls-id <Id>` — Invoke action DropCall

**activity-log** — Manage activity log

- `3cx-xapi-pp-cli activity-log get-filter` — Invoke function GetFilter
- `3cx-xapi-pp-cli activity-log get-logs` — Invoke function GetLogs
- `3cx-xapi-pp-cli activity-log purge-logs` — Invoke action PurgeLogs

**aisettings** — Manage aisettings

- `3cx-xapi-pp-cli aisettings add-vector-store-files` — Invoke action AddVectorStoreFiles
- `3cx-xapi-pp-cli aisettings create-vector-store` — Invoke action CreateVectorStore
- `3cx-xapi-pp-cli aisettings delete-open-ai-file` — Invoke action DeleteOpenAiFile
- `3cx-xapi-pp-cli aisettings delete-vector-store` — Invoke action DeleteVectorStore
- `3cx-xapi-pp-cli aisettings delete-vector-store-file` — Invoke action DeleteVectorStoreFile
- `3cx-xapi-pp-cli aisettings get` — Get AISettings
- `3cx-xapi-pp-cli aisettings get-airesources` — Invoke function GetAIResources
- `3cx-xapi-pp-cli aisettings get-aitemplate-contents` — Invoke function GetAITemplateContents
- `3cx-xapi-pp-cli aisettings get-vector-store` — Invoke function GetVectorStore
- `3cx-xapi-pp-cli aisettings get-vector-store-files` — Invoke function GetVectorStoreFiles
- `3cx-xapi-pp-cli aisettings get-vector-stores` — Invoke function GetVectorStores
- `3cx-xapi-pp-cli aisettings update` — Update AISettings
- `3cx-xapi-pp-cli aisettings update-vector-store` — Invoke action UpdateVectorStore

**amazon-integration-settings** — Manage amazon integration settings

- `3cx-xapi-pp-cli amazon-integration-settings generate-iam-policy-file` — Invoke function GenerateIamPolicyFile
- `3cx-xapi-pp-cli amazon-integration-settings get` — Get AmazonIntegrationSettings
- `3cx-xapi-pp-cli amazon-integration-settings update` — Update AmazonIntegrationSettings

**anti-hacking-settings** — Manage anti hacking settings

- `3cx-xapi-pp-cli anti-hacking-settings get` — Get AntiHackingSettings
- `3cx-xapi-pp-cli anti-hacking-settings update` — Update AntiHackingSettings

**backups** — Manage backups

- `3cx-xapi-pp-cli backups backup` — Invoke action Backup
- `3cx-xapi-pp-cli backups get-can-create` — Invoke function GetCanCreateBackup
- `3cx-xapi-pp-cli backups get-failover-scripts` — Invoke function GetFailoverScripts
- `3cx-xapi-pp-cli backups get-failover-settings` — Invoke function GetBackupFailoverSettings
- `3cx-xapi-pp-cli backups get-local-size` — Invoke function GetLocalBackupSize
- `3cx-xapi-pp-cli backups get-repository-settings` — Invoke function GetBackupRepositorySettings
- `3cx-xapi-pp-cli backups get-restore-settings` — Invoke function GetRestoreSettings
- `3cx-xapi-pp-cli backups get-settings` — Invoke function GetBackupSettings
- `3cx-xapi-pp-cli backups list` — Get entities from Backups
- `3cx-xapi-pp-cli backups purge-local` — Invoke action PurgeLocalBackups
- `3cx-xapi-pp-cli backups set-failover-settings` — Invoke action SetBackupFailoverSettings
- `3cx-xapi-pp-cli backups set-repository-settings` — Invoke action SetBackupRepositorySettings
- `3cx-xapi-pp-cli backups set-restore-settings` — Invoke action SetRestoreSettings
- `3cx-xapi-pp-cli backups set-settings` — Invoke action SetBackupSettings

**backups-file-name** — Manage backups file name

- `3cx-xapi-pp-cli backups-file-name delete-backups` — Delete entity from Backups
- `3cx-xapi-pp-cli backups-file-name get-backup-extras` — Invoke function GetBackupExtras
- `3cx-xapi-pp-cli backups-file-name restore` — Invoke action Restore

**black-list-numbers** — Manage black list numbers

- `3cx-xapi-pp-cli black-list-numbers bulk-numbers-delete` — Invoke action BulkNumbersDelete
- `3cx-xapi-pp-cli black-list-numbers create` — Add new entity to BlackListNumbers
- `3cx-xapi-pp-cli black-list-numbers list` — Get entities from BlackListNumbers

**black-list-numbers-id** — Manage black list numbers id

- `3cx-xapi-pp-cli black-list-numbers-id delete-black-list-number` — Delete entity from BlackListNumbers
- `3cx-xapi-pp-cli black-list-numbers-id get-black-list-number` — Get entity from BlackListNumbers by key
- `3cx-xapi-pp-cli black-list-numbers-id update-black-list-number` — Update entity in BlackListNumbers

**blocklist** — Manage blocklist

- `3cx-xapi-pp-cli blocklist bulk-ips-delete` — Invoke action BulkIpsDelete
- `3cx-xapi-pp-cli blocklist create-addr` — Add new entity to Blocklist
- `3cx-xapi-pp-cli blocklist list-addr` — Get entities from Blocklist

**blocklist-id** — Manage blocklist id

- `3cx-xapi-pp-cli blocklist-id delete-blocklist-addr` — Delete entity from Blocklist
- `3cx-xapi-pp-cli blocklist-id get-blocklist-addr` — Get entity from Blocklist by key
- `3cx-xapi-pp-cli blocklist-id update-blocklist-addr` — Update entity in Blocklist

**call-cost-settings** — Manage call cost settings

- `3cx-xapi-pp-cli call-cost-settings export-call-costs` — Invoke function ExportCallCosts
- `3cx-xapi-pp-cli call-cost-settings list` — Get entities from CallCostSettings
- `3cx-xapi-pp-cli call-cost-settings update-cost` — Invoke action UpdateCost

**call-flow-apps** — Manage call flow apps

- `3cx-xapi-pp-cli call-flow-apps create` — Add new entity to CallFlowApps
- `3cx-xapi-pp-cli call-flow-apps list` — Get entities from CallFlowApps

**call-flow-apps-id** — Manage call flow apps id

- `3cx-xapi-pp-cli call-flow-apps-id delete-call-flow-app` — Delete entity from CallFlowApps
- `3cx-xapi-pp-cli call-flow-apps-id delete-file` — Invoke action DeleteFile
- `3cx-xapi-pp-cli call-flow-apps-id get-call-flow-app` — Get entity from CallFlowApps by key
- `3cx-xapi-pp-cli call-flow-apps-id get-files` — Invoke function GetFiles
- `3cx-xapi-pp-cli call-flow-apps-id update-call-flow-app` — Update entity in CallFlowApps

**call-flow-scripts** — Manage call flow scripts

- `3cx-xapi-pp-cli call-flow-scripts` — Get entities from CallFlowScripts

**call-history-view** — Manage call history view

- `3cx-xapi-pp-cli call-history-view download-call-history` — Invoke function DownloadCallHistory
- `3cx-xapi-pp-cli call-history-view list` — Get entities from CallHistoryView

**call-parking-settings** — Manage call parking settings

- `3cx-xapi-pp-cli call-parking-settings get` — Get CallParkingSettings
- `3cx-xapi-pp-cli call-parking-settings update` — Update CallParkingSettings

**call-types-settings** — Manage call types settings

- `3cx-xapi-pp-cli call-types-settings get` — Get CallTypesSettings
- `3cx-xapi-pp-cli call-types-settings update` — Update CallTypesSettings

**cdrsettings** — Manage cdrsettings

- `3cx-xapi-pp-cli cdrsettings get` — Get CDRSettings
- `3cx-xapi-pp-cli cdrsettings update` — Update CDRSettings

**chat-history-view** — Manage chat history view

- `3cx-xapi-pp-cli chat-history-view download-chat-history` — Invoke function DownloadChatHistory
- `3cx-xapi-pp-cli chat-history-view list` — Get entities from ChatHistoryView

**chat-log-settings** — Manage chat log settings

- `3cx-xapi-pp-cli chat-log-settings get` — Get ChatLogSettings
- `3cx-xapi-pp-cli chat-log-settings update` — Update ChatLogSettings

**chat-messages-history-view** — Manage chat messages history view

- `3cx-xapi-pp-cli chat-messages-history-view download-chat-messages-history` — Invoke function DownloadChatMessagesHistory
- `3cx-xapi-pp-cli chat-messages-history-view list` — Get entities from ChatMessagesHistoryView

**codecs-settings** — Manage codecs settings

- `3cx-xapi-pp-cli codecs-settings get` — Get CodecsSettings
- `3cx-xapi-pp-cli codecs-settings update` — Update CodecsSettings

**conference-settings** — Manage conference settings

- `3cx-xapi-pp-cli conference-settings delete-api-key` — Invoke action DeleteApiKey
- `3cx-xapi-pp-cli conference-settings generate-api-key` — Invoke function GenerateApiKey
- `3cx-xapi-pp-cli conference-settings get` — Get ConferenceSettings
- `3cx-xapi-pp-cli conference-settings get-mcurequest-status` — Invoke function GetMCURequestStatus
- `3cx-xapi-pp-cli conference-settings get-mcurow` — Invoke function GetMCURow
- `3cx-xapi-pp-cli conference-settings get-mcurows` — Invoke function GetMCURows
- `3cx-xapi-pp-cli conference-settings get-onboard-mcu-data` — Invoke function GetOnboardMcuData
- `3cx-xapi-pp-cli conference-settings get-onboard-meetings` — Invoke function GetOnboardMeetings
- `3cx-xapi-pp-cli conference-settings get-web-meeting-zones` — Invoke function GetWebMeetingZones
- `3cx-xapi-pp-cli conference-settings update` — Update ConferenceSettings
- `3cx-xapi-pp-cli conference-settings update-mcurequest-status` — Invoke action UpdateMCURequestStatus

**console-restrictions** — Manage console restrictions

- `3cx-xapi-pp-cli console-restrictions get` — Get ConsoleRestrictions
- `3cx-xapi-pp-cli console-restrictions update` — Update ConsoleRestrictions

**contacts** — Manage contacts

- `3cx-xapi-pp-cli contacts create` — Add new entity to Contacts
- `3cx-xapi-pp-cli contacts create-by-number` — Invoke action CreateContactByNumber
- `3cx-xapi-pp-cli contacts delete-by-id` — Invoke action DeleteContactsById
- `3cx-xapi-pp-cli contacts delete-by-type` — Invoke action DeleteContactsByType
- `3cx-xapi-pp-cli contacts delete-department-by-type` — Invoke action DeleteDepartmentContactsByType
- `3cx-xapi-pp-cli contacts delete-personal-by-type` — Invoke action DeletePersonalContactsByType
- `3cx-xapi-pp-cli contacts get-by-number` — Invoke function GetContactsByNumber
- `3cx-xapi-pp-cli contacts get-dir-search-settings` — Invoke function GetDirSearchSettings
- `3cx-xapi-pp-cli contacts list` — Get entities from Contacts
- `3cx-xapi-pp-cli contacts set-dir-search-settings` — Invoke action SetDirSearchSettings

**contacts-id** — Manage contacts id

- `3cx-xapi-pp-cli contacts-id delete-contact` — Delete entity from Contacts
- `3cx-xapi-pp-cli contacts-id get-contact` — Get entity from Contacts by key
- `3cx-xapi-pp-cli contacts-id update-contact` — Update entity in Contacts

**countries** — Manage countries

- `3cx-xapi-pp-cli countries` — Get entities from Countries

**country-codes** — Manage country codes

- `3cx-xapi-pp-cli country-codes get` — Get CountryCodes
- `3cx-xapi-pp-cli country-codes update` — Update CountryCodes

**crm-integration** — Manage crm integration

- `3cx-xapi-pp-cli crm-integration delete-crm-contacts` — Invoke action DeleteCrmContacts
- `3cx-xapi-pp-cli crm-integration get` — Get CrmIntegration
- `3cx-xapi-pp-cli crm-integration get-crm-template-source` — Invoke function GetCrmTemplateSource
- `3cx-xapi-pp-cli crm-integration get-oauth` — Invoke function GetOAuth
- `3cx-xapi-pp-cli crm-integration set-oauth-state` — Invoke action SetOAuthState
- `3cx-xapi-pp-cli crm-integration test` — Invoke action Test
- `3cx-xapi-pp-cli crm-integration update` — Update CrmIntegration

**crm-templates** — Manage crm templates

- `3cx-xapi-pp-cli crm-templates` — Invoke function GeCrmtTemplates

**crm-templates-name** — Manage crm templates name

- `3cx-xapi-pp-cli crm-templates-name delete-crm-template` — Delete entity from CrmTemplates
- `3cx-xapi-pp-cli crm-templates-name get-crm-template` — Get entity from CrmTemplates by key

**custom-prompts** — Manage custom prompts

- `3cx-xapi-pp-cli custom-prompts list` — Get entities from CustomPrompts
- `3cx-xapi-pp-cli custom-prompts make-call-record` — Invoke action MakeCallRecordCustomPrompt

**custom-prompts-filename** — Manage custom prompts filename

- `3cx-xapi-pp-cli custom-prompts-filename <Filename>` — Delete entity from CustomPrompts

**data-connector-settings** — Manage data connector settings

- `3cx-xapi-pp-cli data-connector-settings get` — Get DataConnectorSettings
- `3cx-xapi-pp-cli data-connector-settings synchronize` — Invoke action Synchronize
- `3cx-xapi-pp-cli data-connector-settings test-data-connector` — Invoke action TestDataConnector
- `3cx-xapi-pp-cli data-connector-settings update` — Update DataConnectorSettings

**defs** — Manage defs

- `3cx-xapi-pp-cli defs get` — Get Defs
- `3cx-xapi-pp-cli defs get-license-parameters` — Invoke function GetLicenseParameters
- `3cx-xapi-pp-cli defs get-routes` — Invoke action GetRoutes
- `3cx-xapi-pp-cli defs get-system-parameters` — Invoke function GetSystemParameters
- `3cx-xapi-pp-cli defs has-system-owner` — Invoke function HasSystemOwner
- `3cx-xapi-pp-cli defs list-codecs` — Get Codecs from Defs
- `3cx-xapi-pp-cli defs list-gateway-parameter-values` — Get GatewayParameterValues from Defs
- `3cx-xapi-pp-cli defs list-gateway-parameters` — Get GatewayParameters from Defs
- `3cx-xapi-pp-cli defs list-time-zones` — Get TimeZones from Defs
- `3cx-xapi-pp-cli defs send-email` — Invoke action SendEmail

**device-infos** — Manage device infos

- `3cx-xapi-pp-cli device-infos` — Get entities from DeviceInfos

**device-infos-id** — Manage device infos id

- `3cx-xapi-pp-cli device-infos-id delete-device-info` — Delete entity from DeviceInfos
- `3cx-xapi-pp-cli device-infos-id get-device-info` — Get entity from DeviceInfos by key
- `3cx-xapi-pp-cli device-infos-id provision` — Invoke action Provision

**dial-code-settings** — Manage dial code settings

- `3cx-xapi-pp-cli dial-code-settings get` — Get DialCodeSettings
- `3cx-xapi-pp-cli dial-code-settings update` — Update DialCodeSettings

**did-numbers** — Manage did numbers

- `3cx-xapi-pp-cli did-numbers` — Get entities from DidNumbers

**dnproperties** — Manage dnproperties

- `3cx-xapi-pp-cli dnproperties create-dnproperty` — Invoke action CreateDNProperty
- `3cx-xapi-pp-cli dnproperties delete-dnproperty` — Invoke action DeleteDNProperty
- `3cx-xapi-pp-cli dnproperties get-dnproperty-by-name` — Invoke function GetDNPropertyByName
- `3cx-xapi-pp-cli dnproperties get-properties-by-dn` — Invoke function GetPropertiesByDn
- `3cx-xapi-pp-cli dnproperties update-dnproperty` — Invoke action UpdateDNProperty

**e164-settings** — Manage e164 settings

- `3cx-xapi-pp-cli e164-settings get` — Get E164Settings
- `3cx-xapi-pp-cli e164-settings update` — Update E164Settings

**email-template** — Manage email template

- `3cx-xapi-pp-cli email-template` — Get entities from EmailTemplate

**email-template-template-path** — Manage email template template path

- `3cx-xapi-pp-cli email-template-template-path get-email-template` — Get entity from EmailTemplate by key
- `3cx-xapi-pp-cli email-template-template-path set-default` — Invoke action SetDefault
- `3cx-xapi-pp-cli email-template-template-path update-email-template` — Update entity in EmailTemplate

**emergency-geo-locations** — Manage emergency geo locations

- `3cx-xapi-pp-cli emergency-geo-locations list` — Get entities from EmergencyGeoLocations
- `3cx-xapi-pp-cli emergency-geo-locations update` — Invoke action Update

**emergency-notifications-settings** — Manage emergency notifications settings

- `3cx-xapi-pp-cli emergency-notifications-settings get` — Get EmergencyNotificationsSettings
- `3cx-xapi-pp-cli emergency-notifications-settings update` — Update EmergencyNotificationsSettings

**event-logs** — Manage event logs

- `3cx-xapi-pp-cli event-logs download` — Invoke function DownloadEventLogs
- `3cx-xapi-pp-cli event-logs list` — Get entities from EventLogs
- `3cx-xapi-pp-cli event-logs purge` — Invoke action PurgeEventLog

**fax** — Manage fax

- `3cx-xapi-pp-cli fax bulk-delete` — Invoke action BulkFaxDelete
- `3cx-xapi-pp-cli fax create` — Add new entity to Fax
- `3cx-xapi-pp-cli fax init` — Invoke function InitFax
- `3cx-xapi-pp-cli fax list` — Get entities from Fax

**fax-id** — Manage fax id

- `3cx-xapi-pp-cli fax-id delete-fax` — Delete entity from Fax
- `3cx-xapi-pp-cli fax-id get-fax` — Get entity from Fax by key
- `3cx-xapi-pp-cli fax-id update-fax` — Update entity in Fax

**fax-number-number** — Manage fax number number

- `3cx-xapi-pp-cli fax-number-number <Number>` — Get entity from Fax by key (Number)

**fax-server-settings** — Manage fax server settings

- `3cx-xapi-pp-cli fax-server-settings clean-up-fax` — Invoke action CleanUpFax
- `3cx-xapi-pp-cli fax-server-settings get` — Get FaxServerSettings
- `3cx-xapi-pp-cli fax-server-settings get-fax-files-size` — Invoke function GetFaxFilesSize
- `3cx-xapi-pp-cli fax-server-settings update` — Update FaxServerSettings

**firewall** — Manage firewall

- `3cx-xapi-pp-cli firewall get-last-result` — Invoke function GetLastResult
- `3cx-xapi-pp-cli firewall get-state` — Get Firewall
- `3cx-xapi-pp-cli firewall start-check` — Invoke action StartCheck
- `3cx-xapi-pp-cli firewall stop-check` — Invoke action StopCheck

**firmwares** — Manage firmwares

- `3cx-xapi-pp-cli firmwares clean-up` — Invoke action CleanUp
- `3cx-xapi-pp-cli firmwares get-state` — Invoke function GetFirmwareState
- `3cx-xapi-pp-cli firmwares list` — Get entities from Firmwares

**firmwares-id** — Manage firmwares id

- `3cx-xapi-pp-cli firmwares-id delete-firmware` — Delete entity from Firmwares
- `3cx-xapi-pp-cli firmwares-id push-firmware-for-phones` — Invoke action PushFirmwareForPhones

**fxs** — Manage fxs

- `3cx-xapi-pp-cli fxs create` — Add new entity to Fxs
- `3cx-xapi-pp-cli fxs list` — Get entities from Fxs

**fxs-mac-address** — Manage fxs mac address

- `3cx-xapi-pp-cli fxs-mac-address delete-fxs` — Delete entity from Fxs
- `3cx-xapi-pp-cli fxs-mac-address get-fxs` — Get entity from Fxs by key
- `3cx-xapi-pp-cli fxs-mac-address regenerate-web-credentials` — Invoke action RegenerateWebCredentials
- `3cx-xapi-pp-cli fxs-mac-address update-fxs` — Update entity in Fxs

**fxs-templates** — Manage fxs templates

- `3cx-xapi-pp-cli fxs-templates create` — Add new entity to FxsTemplates
- `3cx-xapi-pp-cli fxs-templates list` — Get entities from FxsTemplates

**fxs-templates-id** — Manage fxs templates id

- `3cx-xapi-pp-cli fxs-templates-id delete-fxs-template` — Delete entity from FxsTemplates
- `3cx-xapi-pp-cli fxs-templates-id get-fxs-template` — Get entity from FxsTemplates by key
- `3cx-xapi-pp-cli fxs-templates-id update-fxs-template` — Update entity in FxsTemplates

**general-settings-for-apps** — Manage general settings for apps

- `3cx-xapi-pp-cli general-settings-for-apps get` — Get GeneralSettingsForApps
- `3cx-xapi-pp-cli general-settings-for-apps update` — Update GeneralSettingsForApps

**general-settings-for-pbx** — Manage general settings for pbx

- `3cx-xapi-pp-cli general-settings-for-pbx get` — Get GeneralSettingsForPbx
- `3cx-xapi-pp-cli general-settings-for-pbx update` — Update GeneralSettingsForPbx

**get-client-crm-updates** — Manage get client crm updates

- `3cx-xapi-pp-cli get-client-crm-updates` — Invoke functionImport GetClientCrmUpdates

**get-directory-info** — Manage get directory info

- `3cx-xapi-pp-cli get-directory-info` — Invoke actionImport GetDirectoryInfo

**get-prompt-set-updates** — Manage get prompt set updates

- `3cx-xapi-pp-cli get-prompt-set-updates` — Invoke functionImport GetPromptSetUpdates

**get-server-crm-updates** — Manage get server crm updates

- `3cx-xapi-pp-cli get-server-crm-updates` — Invoke functionImport GetServerCrmUpdates

**get-update-settings** — Manage get update settings

- `3cx-xapi-pp-cli get-update-settings` — Invoke functionImport GetUpdateSettings

**get-updates** — Manage get updates

- `3cx-xapi-pp-cli get-updates` — Invoke functionImport GetUpdates

**get-updates-stats** — Manage get updates stats

- `3cx-xapi-pp-cli get-updates-stats` — Invoke functionImport GetUpdatesStats

**google-settings** — Manage google settings

- `3cx-xapi-pp-cli google-settings get` — Get GoogleSettings
- `3cx-xapi-pp-cli google-settings get-google-users` — Invoke action GetGoogleUsers
- `3cx-xapi-pp-cli google-settings update` — Update GoogleSettings

**groups** — Manage groups

- `3cx-xapi-pp-cli groups create` — Add new entity to Groups
- `3cx-xapi-pp-cli groups delete-company-by-id` — Invoke action DeleteCompanyById
- `3cx-xapi-pp-cli groups delete-company-by-number` — Invoke action DeleteCompanyByNumber
- `3cx-xapi-pp-cli groups link-partner` — Invoke action LinkGroupPartner
- `3cx-xapi-pp-cli groups list` — Get entities from Groups
- `3cx-xapi-pp-cli groups replace-license-key` — Invoke action ReplaceGroupLicenseKey
- `3cx-xapi-pp-cli groups unlink-partner` — Invoke action UnlinkGroupPartner

**groups-id** — Manage groups id

- `3cx-xapi-pp-cli groups-id delete-group` — Delete entity from Groups
- `3cx-xapi-pp-cli groups-id get-group` — Get entity from Groups by key
- `3cx-xapi-pp-cli groups-id get-restrictions` — Invoke function GetRestrictions
- `3cx-xapi-pp-cli groups-id list-members` — Get Members from Groups
- `3cx-xapi-pp-cli groups-id list-rights` — Get Rights from Groups
- `3cx-xapi-pp-cli groups-id update-group` — Update entity in Groups

**has-debian-upgrade** — Manage has debian upgrade

- `3cx-xapi-pp-cli has-debian-upgrade` — Invoke functionImport HasDebianUpgrade

**holidays** — Manage holidays

- `3cx-xapi-pp-cli holidays create` — Add new entity to Holidays
- `3cx-xapi-pp-cli holidays list` — Get entities from Holidays

**holidays-id** — Manage holidays id

- `3cx-xapi-pp-cli holidays-id delete-holiday` — Delete entity from Holidays
- `3cx-xapi-pp-cli holidays-id get-holiday` — Get entity from Holidays by key
- `3cx-xapi-pp-cli holidays-id update-holiday` — Update entity in Holidays

**hotel-services** — Manage hotel services

- `3cx-xapi-pp-cli hotel-services get` — Get HotelServices
- `3cx-xapi-pp-cli hotel-services update` — Update HotelServices

**inbound-rules** — Manage inbound rules

- `3cx-xapi-pp-cli inbound-rules bulk-delete` — Invoke action BulkInboundRulesDelete
- `3cx-xapi-pp-cli inbound-rules create` — Add new entity to InboundRules
- `3cx-xapi-pp-cli inbound-rules export-caller-id-rules` — Invoke function ExportCallerIdRules
- `3cx-xapi-pp-cli inbound-rules list` — Get entities from InboundRules

**inbound-rules-id** — Manage inbound rules id

- `3cx-xapi-pp-cli inbound-rules-id delete-inbound-rule` — Delete entity from InboundRules
- `3cx-xapi-pp-cli inbound-rules-id get-inbound-rule` — Get entity from InboundRules by key
- `3cx-xapi-pp-cli inbound-rules-id update-inbound-rule` — Update entity in InboundRules

**install-updates** — Manage install updates

- `3cx-xapi-pp-cli install-updates` — Invoke actionImport InstallUpdates

**last-cdr-and-chat-message-timestamp** — Manage last cdr and chat message timestamp

- `3cx-xapi-pp-cli last-cdr-and-chat-message-timestamp` — Get entities from LastCdrAndChatMessageTimestamp

**license-status** — Manage license status

- `3cx-xapi-pp-cli license-status edit-license-info` — Invoke action EditLicenseInfo
- `3cx-xapi-pp-cli license-status get` — Get LicenseStatus
- `3cx-xapi-pp-cli license-status link-partner` — Invoke action LinkPartner
- `3cx-xapi-pp-cli license-status partner-info` — Invoke function PartnerInfo
- `3cx-xapi-pp-cli license-status refresh` — Invoke action RefreshLicenseStatus
- `3cx-xapi-pp-cli license-status replace-license-key` — Invoke action ReplaceLicenseKey
- `3cx-xapi-pp-cli license-status unlink-partner` — Invoke action UnlinkPartner

**logging-settings** — Manage logging settings

- `3cx-xapi-pp-cli logging-settings get` — Get LoggingSettings
- `3cx-xapi-pp-cli logging-settings update` — Update LoggingSettings

**mail-settings** — Manage mail settings

- `3cx-xapi-pp-cli mail-settings get` — Get MailSettings
- `3cx-xapi-pp-cli mail-settings test-email` — Invoke action TestEmail
- `3cx-xapi-pp-cli mail-settings update` — Update MailSettings

**microsoft365-integration** — Manage microsoft365 integration

- `3cx-xapi-pp-cli microsoft365-integration authorize-presence` — Invoke action AuthorizePresence
- `3cx-xapi-pp-cli microsoft365-integration deauthorize-presence` — Invoke action DeauthorizePresence
- `3cx-xapi-pp-cli microsoft365-integration get` — Get Microsoft365Integration
- `3cx-xapi-pp-cli microsoft365-integration get-m365-groups` — Invoke action GetM365Groups
- `3cx-xapi-pp-cli microsoft365-integration get-m365-users` — Invoke action GetM365Users
- `3cx-xapi-pp-cli microsoft365-integration get-microsoft-access-token` — Invoke function GetMicrosoftAccessToken
- `3cx-xapi-pp-cli microsoft365-integration get-microsoft365-directory` — Invoke function GetMicrosoft365Directory
- `3cx-xapi-pp-cli microsoft365-integration get-users-by-principal-names` — Invoke action GetUsersByPrincipalNames
- `3cx-xapi-pp-cli microsoft365-integration test-presence` — Invoke action TestPresence
- `3cx-xapi-pp-cli microsoft365-integration test-subscription` — Invoke function TestSubscription
- `3cx-xapi-pp-cli microsoft365-integration update` — Update Microsoft365Integration

**microsoft365-teams-integration** — Manage microsoft365 teams integration

- `3cx-xapi-pp-cli microsoft365-teams-integration check-fqdn-record` — Invoke function CheckFqdnRecord
- `3cx-xapi-pp-cli microsoft365-teams-integration check-map-users-script` — Invoke function CheckMapUsersScript
- `3cx-xapi-pp-cli microsoft365-teams-integration get` — Get Microsoft365TeamsIntegration
- `3cx-xapi-pp-cli microsoft365-teams-integration get-dial-plan-script` — Invoke function GetDialPlanScript
- `3cx-xapi-pp-cli microsoft365-teams-integration get-map-users-script` — Invoke function GetMapUsersScript
- `3cx-xapi-pp-cli microsoft365-teams-integration update` — Update Microsoft365TeamsIntegration

**music-on-hold-settings** — Manage music on hold settings

- `3cx-xapi-pp-cli music-on-hold-settings get` — Get MusicOnHoldSettings
- `3cx-xapi-pp-cli music-on-hold-settings update` — Update MusicOnHoldSettings

**my-group** — Manage my group

- `3cx-xapi-pp-cli my-group get` — Get MyGroup
- `3cx-xapi-pp-cli my-group get-partner-info` — Invoke function GetMyGroupPartnerInfo
- `3cx-xapi-pp-cli my-group get-restrictions` — Invoke function GetRestrictions
- `3cx-xapi-pp-cli my-group link-partner` — Invoke action LinkMyGroupPartner
- `3cx-xapi-pp-cli my-group list-members` — Get Members from MyGroup
- `3cx-xapi-pp-cli my-group list-rights` — Get Rights from MyGroup
- `3cx-xapi-pp-cli my-group replace-license-key` — Invoke action ReplaceMyGroupLicenseKey
- `3cx-xapi-pp-cli my-group unlink-partner` — Invoke action UnlinkMyGroupPartner
- `3cx-xapi-pp-cli my-group update` — Update MyGroup

**my-tokens** — Manage my tokens

- `3cx-xapi-pp-cli my-tokens` — Get entities from MyTokens

**my-tokens-id** — Manage my tokens id

- `3cx-xapi-pp-cli my-tokens-id <Id>` — Invoke action RevokeToken

**my-user** — Manage my user

- `3cx-xapi-pp-cli my-user generate-prov-link` — Invoke function GenerateProvLink
- `3cx-xapi-pp-cli my-user get` — Get MyUser
- `3cx-xapi-pp-cli my-user list-forwarding-profiles` — Get ForwardingProfiles from MyUser
- `3cx-xapi-pp-cli my-user list-greetings` — Get Greetings from MyUser
- `3cx-xapi-pp-cli my-user list-groups` — Get Groups from MyUser
- `3cx-xapi-pp-cli my-user update` — Update MyUser

**network-interfaces** — Manage network interfaces

- `3cx-xapi-pp-cli network-interfaces` — Get entities from NetworkInterfaces

**network-settings** — Manage network settings

- `3cx-xapi-pp-cli network-settings get` — Get NetworkSettings
- `3cx-xapi-pp-cli network-settings get-ifaces` — Invoke function GetIfaces
- `3cx-xapi-pp-cli network-settings update` — Update NetworkSettings

**notification-settings** — Manage notification settings

- `3cx-xapi-pp-cli notification-settings get` — Get NotificationSettings
- `3cx-xapi-pp-cli notification-settings update` — Update NotificationSettings

**office-hours** — Manage office hours

- `3cx-xapi-pp-cli office-hours get` — Get OfficeHours
- `3cx-xapi-pp-cli office-hours update` — Update OfficeHours

**outbound-rules** — Manage outbound rules

- `3cx-xapi-pp-cli outbound-rules create` — Add new entity to OutboundRules
- `3cx-xapi-pp-cli outbound-rules get-emergency` — Invoke function GetEmergencyOutboundRules
- `3cx-xapi-pp-cli outbound-rules list` — Get entities from OutboundRules
- `3cx-xapi-pp-cli outbound-rules move-up-down` — Invoke action MoveUpDown
- `3cx-xapi-pp-cli outbound-rules purge` — Invoke action Purge

**outbound-rules-id** — Manage outbound rules id

- `3cx-xapi-pp-cli outbound-rules-id delete-outbound-rule` — Delete entity from OutboundRules
- `3cx-xapi-pp-cli outbound-rules-id get-outbound-rule` — Get entity from OutboundRules by key
- `3cx-xapi-pp-cli outbound-rules-id update-outbound-rule` — Update entity in OutboundRules

**parameters** — Manage parameters

- `3cx-xapi-pp-cli parameters create` — Add new entity to Parameters
- `3cx-xapi-pp-cli parameters get-by-name` — Invoke function GetParameterByName
- `3cx-xapi-pp-cli parameters list` — Get entities from Parameters

**parameters-id** — Manage parameters id

- `3cx-xapi-pp-cli parameters-id delete-parameter` — Delete entity from Parameters
- `3cx-xapi-pp-cli parameters-id get-parameter` — Get entity from Parameters by key
- `3cx-xapi-pp-cli parameters-id update-parameter` — Update entity in Parameters

**parkings** — Manage parkings

- `3cx-xapi-pp-cli parkings create` — Add new entity to Parkings
- `3cx-xapi-pp-cli parkings list` — Get entities from Parkings

**parkings-id** — Manage parkings id

- `3cx-xapi-pp-cli parkings-id delete-parking` — Delete entity from Parkings
- `3cx-xapi-pp-cli parkings-id get-parking` — Get entity from Parkings by key
- `3cx-xapi-pp-cli parkings-id list-parking-groups` — Get Groups from Parkings
- `3cx-xapi-pp-cli parkings-id update-parking` — Update entity in Parkings

**parkings-number-number** — Manage parkings number number

- `3cx-xapi-pp-cli parkings-number-number <Number>` — Get entity from Parkings by key (Number)

**peers** — Manage peers

- `3cx-xapi-pp-cli peers get-report` — Invoke function GetReportPeers
- `3cx-xapi-pp-cli peers list` — Get entities from Peers
- `3cx-xapi-pp-cli peers retreive-by-numbers` — Invoke action RetreivePeersByNumbers

**peers-number-number** — Manage peers number number

- `3cx-xapi-pp-cli peers-number-number <Number>` — Get entity from Peers by key (Number)

**phone-book-settings** — Manage phone book settings

- `3cx-xapi-pp-cli phone-book-settings get` — Get PhoneBookSettings
- `3cx-xapi-pp-cli phone-book-settings update` — Update PhoneBookSettings

**phone-logos** — Manage phone logos

- `3cx-xapi-pp-cli phone-logos` — Get entities from PhoneLogos

**phone-logos-filename** — Manage phone logos filename

- `3cx-xapi-pp-cli phone-logos-filename <Filename>` — Delete entity from PhoneLogos

**phone-templates** — Manage phone templates

- `3cx-xapi-pp-cli phone-templates create` — Add new entity to PhoneTemplates
- `3cx-xapi-pp-cli phone-templates list` — Get entities from PhoneTemplates

**phone-templates-id** — Manage phone templates id

- `3cx-xapi-pp-cli phone-templates-id delete-phone-template` — Delete entity from PhoneTemplates
- `3cx-xapi-pp-cli phone-templates-id get-phone-template` — Get entity from PhoneTemplates by key
- `3cx-xapi-pp-cli phone-templates-id update-phone-template` — Update entity in PhoneTemplates

**phones-settings** — Manage phones settings

- `3cx-xapi-pp-cli phones-settings get` — Get PhonesSettings
- `3cx-xapi-pp-cli phones-settings update` — Update PhonesSettings

**playlists** — Manage playlists

- `3cx-xapi-pp-cli playlists create` — Add new entity to Playlists
- `3cx-xapi-pp-cli playlists delete-file` — Invoke action DeletePlaylistFile
- `3cx-xapi-pp-cli playlists download-file` — Invoke function DownloadPlaylistFile
- `3cx-xapi-pp-cli playlists list` — Get entities from Playlists

**playlists-name** — Manage playlists name

- `3cx-xapi-pp-cli playlists-name delete-playlist` — Delete entity from Playlists
- `3cx-xapi-pp-cli playlists-name get-playlist` — Get entity from Playlists by key
- `3cx-xapi-pp-cli playlists-name update-playlist` — Update entity in Playlists

**prompt-sets** — Manage prompt sets

- `3cx-xapi-pp-cli prompt-sets get-active` — Invoke function GetActive
- `3cx-xapi-pp-cli prompt-sets list` — Get entities from PromptSets

**prompt-sets-id** — Manage prompt sets id

- `3cx-xapi-pp-cli prompt-sets-id copy` — Invoke action Copy
- `3cx-xapi-pp-cli prompt-sets-id delete-prompt-set` — Delete entity from PromptSets
- `3cx-xapi-pp-cli prompt-sets-id get-prompt-set` — Get entity from PromptSets by key
- `3cx-xapi-pp-cli prompt-sets-id list-prompts` — Get Prompts from PromptSets
- `3cx-xapi-pp-cli prompt-sets-id play-prompt` — Invoke action PlayPrompt
- `3cx-xapi-pp-cli prompt-sets-id record-prompt` — Invoke action RecordPrompt
- `3cx-xapi-pp-cli prompt-sets-id set-active` — Invoke action SetActive
- `3cx-xapi-pp-cli prompt-sets-id set-alternate-pronunciation` — Invoke action SetAlternatePronunciation
- `3cx-xapi-pp-cli prompt-sets-id update-prompt-set` — Update entity in PromptSets

**purge-all-logs** — Manage purge all logs

- `3cx-xapi-pp-cli purge-all-logs` — Invoke actionImport PurgeAllLogs

**purge-calls** — Manage purge calls

- `3cx-xapi-pp-cli purge-calls` — Invoke actionImport PurgeCalls

**purge-chats** — Manage purge chats

- `3cx-xapi-pp-cli purge-chats` — Invoke actionImport PurgeChats

**queues** — Manage queues

- `3cx-xapi-pp-cli queues create` — Add new entity to Queues
- `3cx-xapi-pp-cli queues get-first-available-number` — Invoke function GetFirstAvailableQueueNumber
- `3cx-xapi-pp-cli queues list` — Get entities from Queues

**queues-id** — Manage queues id

- `3cx-xapi-pp-cli queues-id delete-queue` — Delete entity from Queues
- `3cx-xapi-pp-cli queues-id get-queue` — Get entity from Queues by key
- `3cx-xapi-pp-cli queues-id list-agents` — Get Agents from Queues
- `3cx-xapi-pp-cli queues-id list-managers` — Get Managers from Queues
- `3cx-xapi-pp-cli queues-id reset-queue-statistics` — Invoke action ResetQueueStatistics
- `3cx-xapi-pp-cli queues-id update-queue` — Update entity in Queues

**queues-number-number** — Manage queues number number

- `3cx-xapi-pp-cli queues-number-number <Number>` — Get entity from Queues by key (Number)

**receptionists** — Manage receptionists

- `3cx-xapi-pp-cli receptionists create` — Add new entity to Receptionists
- `3cx-xapi-pp-cli receptionists get-first-available-number` — Invoke function GetFirstAvailableReceptionistNumber
- `3cx-xapi-pp-cli receptionists list` — Get entities from Receptionists

**receptionists-id** — Manage receptionists id

- `3cx-xapi-pp-cli receptionists-id delete-receptionist` — Delete entity from Receptionists
- `3cx-xapi-pp-cli receptionists-id get-receptionist` — Get entity from Receptionists by key
- `3cx-xapi-pp-cli receptionists-id list-forwards` — Get Forwards from Receptionists
- `3cx-xapi-pp-cli receptionists-id update-receptionist` — Update entity in Receptionists

**receptionists-number-number** — Manage receptionists number number

- `3cx-xapi-pp-cli receptionists-number-number <Number>` — Get entity from Receptionists by key (Number)

**recordings** — Manage recordings

- `3cx-xapi-pp-cli recordings archive` — Invoke action Archive
- `3cx-xapi-pp-cli recordings bulk-archive` — Invoke action BulkRecordingsArchive
- `3cx-xapi-pp-cli recordings bulk-delete` — Invoke action BulkRecordingsDelete
- `3cx-xapi-pp-cli recordings download` — Invoke function DownloadRecording
- `3cx-xapi-pp-cli recordings get-repository-settings` — Invoke function GetRecordingRepositorySettings
- `3cx-xapi-pp-cli recordings list` — Get entities from Recordings
- `3cx-xapi-pp-cli recordings purge-archive` — Invoke action PurgeArchive
- `3cx-xapi-pp-cli recordings purge-local` — Invoke action PurgeLocal
- `3cx-xapi-pp-cli recordings set-repository-settings` — Invoke action SetRecordingRepositorySettings
- `3cx-xapi-pp-cli recordings transcribe` — Invoke action TranscribeRecordings

**remote-archiving-settings** — Manage remote archiving settings

- `3cx-xapi-pp-cli remote-archiving-settings archive-chats` — Invoke action ArchiveChats
- `3cx-xapi-pp-cli remote-archiving-settings archive-faxes` — Invoke action ArchiveFaxes
- `3cx-xapi-pp-cli remote-archiving-settings archive-recordings` — Invoke action ArchiveRecordings
- `3cx-xapi-pp-cli remote-archiving-settings archive-voicemail` — Invoke action ArchiveVoicemail
- `3cx-xapi-pp-cli remote-archiving-settings get` — Get RemoteArchivingSettings
- `3cx-xapi-pp-cli remote-archiving-settings update` — Update RemoteArchivingSettings

**report-abandoned-chats-statistics** — Manage report abandoned chats statistics

- `3cx-xapi-pp-cli report-abandoned-chats-statistics download-abandoned-chats-statistics` — Invoke function DownloadAbandonedChatsStatistics
- `3cx-xapi-pp-cli report-abandoned-chats-statistics get-abandoned-chats-statistics-data` — Invoke function GetAbandonedChatsStatisticsData

**report-abandoned-queue-calls** — Manage report abandoned queue calls

- `3cx-xapi-pp-cli report-abandoned-queue-calls download-abandoned-queue-calls` — Invoke function DownloadAbandonedQueueCalls
- `3cx-xapi-pp-cli report-abandoned-queue-calls get-abandoned-queue-calls-data` — Invoke function GetAbandonedQueueCallsData

**report-agent-login-history** — Manage report agent login history

- `3cx-xapi-pp-cli report-agent-login-history download-agent-login-history` — Invoke function DownloadAgentLoginHistory
- `3cx-xapi-pp-cli report-agent-login-history get-agent-login-history-data` — Invoke function GetAgentLoginHistoryData

**report-agents-in-queue-statistics** — Manage report agents in queue statistics

- `3cx-xapi-pp-cli report-agents-in-queue-statistics download-agents-in-queue-statistics` — Invoke function DownloadAgentsInQueueStatistics
- `3cx-xapi-pp-cli report-agents-in-queue-statistics get-agents-in-queue-statistics-data` — Invoke function GetAgentsInQueueStatisticsData

**report-audit-log** — Manage report audit log

- `3cx-xapi-pp-cli report-audit-log download-audit-log` — Invoke function DownloadAuditLog
- `3cx-xapi-pp-cli report-audit-log get-audit-log-data` — Invoke function GetAuditLogData

**report-average-queue-waiting-time** — Manage report average queue waiting time

- `3cx-xapi-pp-cli report-average-queue-waiting-time download-average-queue-waiting-time-report` — Invoke function DownloadAverageQueueWaitingTimeReport
- `3cx-xapi-pp-cli report-average-queue-waiting-time get-average-queue-waiting-time-data` — Invoke function GetAverageQueueWaitingTimeData

**report-breaches-sla** — Manage report breaches sla

- `3cx-xapi-pp-cli report-breaches-sla download-breaches-sla` — Invoke function DownloadBreachesSla
- `3cx-xapi-pp-cli report-breaches-sla get-breaches-sla-data` — Invoke function GetBreachesSlaData

**report-call-cost-by-extension-group** — Manage report call cost by extension group

- `3cx-xapi-pp-cli report-call-cost-by-extension-group download-call-cost-by-extension-group` — Invoke function DownloadCallCostByExtensionGroup
- `3cx-xapi-pp-cli report-call-cost-by-extension-group get-call-cost-by-extension-group-data` — Invoke function GetCallCostByExtensionGroupData

**report-call-distribution** — Manage report call distribution

- `3cx-xapi-pp-cli report-call-distribution download-get-get-call-distribution` — Invoke function DownloadGetGetCallDistribution
- `3cx-xapi-pp-cli report-call-distribution get-call-distribution` — Invoke function GetCallDistribution

**report-call-log-data** — Manage report call log data

- `3cx-xapi-pp-cli report-call-log-data download-call-log` — Invoke function DownloadCallLog
- `3cx-xapi-pp-cli report-call-log-data get-call-log-data` — Invoke function GetCallLogData
- `3cx-xapi-pp-cli report-call-log-data get-call-quality-report` — Invoke function GetCallQualityReport
- `3cx-xapi-pp-cli report-call-log-data get-old-call-log-data` — Invoke function GetOldCallLogData
- `3cx-xapi-pp-cli report-call-log-data get-old-call-quality-report` — Invoke function GetOldCallQualityReport

**report-chat-log** — Manage report chat log

- `3cx-xapi-pp-cli report-chat-log download-chat-log` — Invoke function DownloadChatLog
- `3cx-xapi-pp-cli report-chat-log get-chat-log` — Invoke function GetChatLog

**report-detailed-queue-statistics** — Manage report detailed queue statistics

- `3cx-xapi-pp-cli report-detailed-queue-statistics download-detailed-queue-statistics` — Invoke function DownloadDetailedQueueStatistics
- `3cx-xapi-pp-cli report-detailed-queue-statistics get-detailed-queue-statistics-data` — Invoke function GetDetailedQueueStatisticsData

**report-extension-statistics** — Manage report extension statistics

- `3cx-xapi-pp-cli report-extension-statistics download-extension-statistics` — Invoke function DownloadExtensionStatistics
- `3cx-xapi-pp-cli report-extension-statistics get-extension-statistics-data` — Invoke function GetExtensionStatisticsData

**report-extension-statistics-by-group** — Manage report extension statistics by group

- `3cx-xapi-pp-cli report-extension-statistics-by-group download-extension-statistics-by-group` — Invoke function DownloadExtensionStatisticsByGroup
- `3cx-xapi-pp-cli report-extension-statistics-by-group get-extension-statistics-by-group-data` — Invoke function GetExtensionStatisticsByGroupData

**report-extensions-statistics-by-ring-groups** — Manage report extensions statistics by ring groups

- `3cx-xapi-pp-cli report-extensions-statistics-by-ring-groups download-extensions-statistics-by-ring-groups` — Invoke function DownloadExtensionsStatisticsByRingGroups
- `3cx-xapi-pp-cli report-extensions-statistics-by-ring-groups get-extensions-statistics-by-ring-groups-data` — Invoke function GetExtensionsStatisticsByRingGroupsData

**report-inbound-calls** — Manage report inbound calls

- `3cx-xapi-pp-cli report-inbound-calls download-get-inbound-calls` — Invoke function DownloadGetInboundCalls
- `3cx-xapi-pp-cli report-inbound-calls get-inbound-calls` — Invoke function GetInboundCalls

**report-inbound-rules** — Manage report inbound rules

- `3cx-xapi-pp-cli report-inbound-rules download-inbound-rules` — Invoke function DownloadInboundRules
- `3cx-xapi-pp-cli report-inbound-rules get-inbound-rules-data` — Invoke function GetInboundRulesData

**report-outbound-calls** — Manage report outbound calls

- `3cx-xapi-pp-cli report-outbound-calls download-get-outbound-calls` — Invoke function DownloadGetOutboundCalls
- `3cx-xapi-pp-cli report-outbound-calls get-outbound-calls` — Invoke function GetOutboundCalls

**report-queue-agents-chat-statistics** — Manage report queue agents chat statistics

- `3cx-xapi-pp-cli report-queue-agents-chat-statistics download-queue-agents-chat-statistics` — Invoke function DownloadQueueAgentsChatStatistics
- `3cx-xapi-pp-cli report-queue-agents-chat-statistics get-queue-agents-chat-statistics-data` — Invoke function GetQueueAgentsChatStatisticsData

**report-queue-agents-chat-statistics-totals** — Manage report queue agents chat statistics totals

- `3cx-xapi-pp-cli report-queue-agents-chat-statistics-totals download-queue-agents-chat-statistics-totals` — Invoke function DownloadQueueAgentsChatStatisticsTotals
- `3cx-xapi-pp-cli report-queue-agents-chat-statistics-totals get-queue-agents-chat-statistics-totals-data` — Invoke function GetQueueAgentsChatStatisticsTotalsData

**report-queue-an-un-calls** — Manage report queue an un calls

- `3cx-xapi-pp-cli report-queue-an-un-calls download-queue-an-un-calls-report` — Invoke function DownloadQueueAnUnCallsReport
- `3cx-xapi-pp-cli report-queue-an-un-calls get-queue-an-un-calls-data` — Invoke function GetQueueAnUnCallsData

**report-queue-answered-calls-by-wait-time** — Manage report queue answered calls by wait time

- `3cx-xapi-pp-cli report-queue-answered-calls-by-wait-time download-queue-answered-calls-by-wait-time` — Invoke function DownloadQueueAnsweredCallsByWaitTime
- `3cx-xapi-pp-cli report-queue-answered-calls-by-wait-time get-queue-answered-calls-by-wait-time-data` — Invoke function GetQueueAnsweredCallsByWaitTimeData

**report-queue-callbacks** — Manage report queue callbacks

- `3cx-xapi-pp-cli report-queue-callbacks download-queue-callbacks` — Invoke function DownloadQueueCallbacks
- `3cx-xapi-pp-cli report-queue-callbacks get-queue-callbacks-data` — Invoke function GetQueueCallbacksData

**report-queue-chat-performance** — Manage report queue chat performance

- `3cx-xapi-pp-cli report-queue-chat-performance download-queue-chat-performance` — Invoke function DownloadQueueChatPerformance
- `3cx-xapi-pp-cli report-queue-chat-performance get-queue-chat-performance-data` — Invoke function GetQueueChatPerformanceData

**report-queue-failed-callbacks** — Manage report queue failed callbacks

- `3cx-xapi-pp-cli report-queue-failed-callbacks download-queue-failed-callbacks` — Invoke function DownloadQueueFailedCallbacks
- `3cx-xapi-pp-cli report-queue-failed-callbacks get-queue-failed-callbacks-data` — Invoke function GetQueueFailedCallbacksData

**report-queue-performance-overview** — Manage report queue performance overview

- `3cx-xapi-pp-cli report-queue-performance-overview download-queue-performance-overview` — Invoke function DownloadQueuePerformanceOverview
- `3cx-xapi-pp-cli report-queue-performance-overview get-queue-performance-overview-data` — Invoke function GetQueuePerformanceOverviewData

**report-queue-performance-totals** — Manage report queue performance totals

- `3cx-xapi-pp-cli report-queue-performance-totals download-queue-performance-totals` — Invoke function DownloadQueuePerformanceTotals
- `3cx-xapi-pp-cli report-queue-performance-totals get-queue-performance-totals-data` — Invoke function GetQueuePerformanceTotalsData

**report-ring-group-statistics** — Manage report ring group statistics

- `3cx-xapi-pp-cli report-ring-group-statistics download-ring-group-statistics` — Invoke function DownloadRingGroupStatistics
- `3cx-xapi-pp-cli report-ring-group-statistics get-ring-group-statistics-data` — Invoke function GetRingGroupStatisticsData

**report-statistic-sla** — Manage report statistic sla

- `3cx-xapi-pp-cli report-statistic-sla download-statistic-sla` — Invoke function DownloadStatisticSla
- `3cx-xapi-pp-cli report-statistic-sla get-statistic-sla-data` — Invoke function GetStatisticSlaData

**report-team-queue-general-statistics** — Manage report team queue general statistics

- `3cx-xapi-pp-cli report-team-queue-general-statistics download-team-queue-general-statistics` — Invoke function DownloadTeamQueueGeneralStatistics
- `3cx-xapi-pp-cli report-team-queue-general-statistics get-team-queue-general-statistics-data` — Invoke function GetTeamQueueGeneralStatisticsData

**report-user-activity** — Manage report user activity

- `3cx-xapi-pp-cli report-user-activity download-get-user-activity` — Invoke function DownloadGetUserActivity
- `3cx-xapi-pp-cli report-user-activity get-user-activity` — Invoke function GetUserActivity

**ring-groups** — Manage ring groups

- `3cx-xapi-pp-cli ring-groups create` — Add new entity to RingGroups
- `3cx-xapi-pp-cli ring-groups get-first-available-number` — Invoke function GetFirstAvailableRingGroupNumber
- `3cx-xapi-pp-cli ring-groups list` — Get entities from RingGroups

**ring-groups-id** — Manage ring groups id

- `3cx-xapi-pp-cli ring-groups-id delete-ring-group` — Delete entity from RingGroups
- `3cx-xapi-pp-cli ring-groups-id get-ring-group` — Get entity from RingGroups by key
- `3cx-xapi-pp-cli ring-groups-id list-ring-group-members` — Get Members from RingGroups
- `3cx-xapi-pp-cli ring-groups-id update-ring-group` — Update entity in RingGroups

**ring-groups-number-number** — Manage ring groups number number

- `3cx-xapi-pp-cli ring-groups-number-number <Number>` — Get entity from RingGroups by key (Number)

**sbcs** — Manage sbcs

- `3cx-xapi-pp-cli sbcs create` — Add new entity to Sbcs
- `3cx-xapi-pp-cli sbcs list` — Get entities from Sbcs

**sbcs-name** — Manage sbcs name

- `3cx-xapi-pp-cli sbcs-name delete-sbc` — Delete entity from Sbcs
- `3cx-xapi-pp-cli sbcs-name get-sbc` — Get entity from Sbcs by key
- `3cx-xapi-pp-cli sbcs-name push-config` — Invoke action PushConfig
- `3cx-xapi-pp-cli sbcs-name update-sbc` — Update entity in Sbcs

**scheduled-reports** — Manage scheduled reports

- `3cx-xapi-pp-cli scheduled-reports create` — Add new entity to ScheduledReports
- `3cx-xapi-pp-cli scheduled-reports list` — Get entities from ScheduledReports

**scheduled-reports-id** — Manage scheduled reports id

- `3cx-xapi-pp-cli scheduled-reports-id delete-scheduled-report` — Delete entity from ScheduledReports
- `3cx-xapi-pp-cli scheduled-reports-id get-scheduled-report` — Get entity from ScheduledReports by key
- `3cx-xapi-pp-cli scheduled-reports-id update-scheduled-report` — Update entity in ScheduledReports

**secure-sip-settings** — Manage secure sip settings

- `3cx-xapi-pp-cli secure-sip-settings get` — Get SecureSipSettings
- `3cx-xapi-pp-cli secure-sip-settings update` — Update SecureSipSettings

**security-tokens** — Manage security tokens

- `3cx-xapi-pp-cli security-tokens` — Get entities from SecurityTokens

**security-tokens-id** — Manage security tokens id

- `3cx-xapi-pp-cli security-tokens-id <Id>` — Invoke action RevokeToken

**service-principals** — Manage service principals

- `3cx-xapi-pp-cli service-principals create` — Add new entity to ServicePrincipals
- `3cx-xapi-pp-cli service-principals list` — Get entities from ServicePrincipals

**service-principals-id** — Manage service principals id

- `3cx-xapi-pp-cli service-principals-id delete-service-principal` — Delete entity from ServicePrincipals
- `3cx-xapi-pp-cli service-principals-id generate-app-token` — Invoke action GenerateAppToken
- `3cx-xapi-pp-cli service-principals-id get-service-principal` — Get entity from ServicePrincipals by key
- `3cx-xapi-pp-cli service-principals-id update-service-principal` — Update entity in ServicePrincipals

**services** — Manage services

- `3cx-xapi-pp-cli services disable` — Invoke action Disable
- `3cx-xapi-pp-cli services enable` — Invoke action Enable
- `3cx-xapi-pp-cli services garbage-collect` — Invoke action GarbageCollect
- `3cx-xapi-pp-cli services list-info` — Get entities from Services
- `3cx-xapi-pp-cli services restart` — Invoke action Restart
- `3cx-xapi-pp-cli services restart-operating-system` — Invoke action RestartOperatingSystem
- `3cx-xapi-pp-cli services start` — Invoke action Start
- `3cx-xapi-pp-cli services stop` — Invoke action Stop

**set-update-settings** — Manage set update settings

- `3cx-xapi-pp-cli set-update-settings` — Invoke actionImport SetUpdateSettings

**sip-devices** — Manage sip devices

- `3cx-xapi-pp-cli sip-devices` — Get entities from SipDevices

**sip-devices-id** — Manage sip devices id

- `3cx-xapi-pp-cli sip-devices-id <Id>` — Invoke action PushFirmware

**syslog-settings** — Manage syslog settings

- `3cx-xapi-pp-cli syslog-settings get` — Get SyslogSettings
- `3cx-xapi-pp-cli syslog-settings test-connection` — Invoke action TestConnection
- `3cx-xapi-pp-cli syslog-settings update` — Update SyslogSettings

**system-status** — Manage system status

- `3cx-xapi-pp-cli system-status apitoken` — Invoke function APIToken
- `3cx-xapi-pp-cli system-status get` — Get SystemStatus
- `3cx-xapi-pp-cli system-status get-remote-access-status` — Invoke function GetRemoteAccessStatus
- `3cx-xapi-pp-cli system-status get-request-help-link` — Invoke function GetRequestHelpLink
- `3cx-xapi-pp-cli system-status get-version-type` — Invoke function GetVersionType
- `3cx-xapi-pp-cli system-status is-request-help-enabled` — Invoke function IsRequestHelpEnabled
- `3cx-xapi-pp-cli system-status my-phone-request-id-telemetry` — Invoke function MyPhoneRequestIdTelemetry
- `3cx-xapi-pp-cli system-status network-telemetry` — Invoke function NetworkTelemetry
- `3cx-xapi-pp-cli system-status request-help` — Invoke action RequestHelp
- `3cx-xapi-pp-cli system-status revoke-remote-access` — Invoke action RevokeRemoteAccess
- `3cx-xapi-pp-cli system-status service-telemetry` — Invoke function ServiceTelemetry
- `3cx-xapi-pp-cli system-status set-chat-log-status` — Invoke action SetChatLogStatus
- `3cx-xapi-pp-cli system-status set-multi-company-mode` — Invoke action SetMultiCompanyMode
- `3cx-xapi-pp-cli system-status start-dbmaintenance` — Invoke action StartDBMaintenance
- `3cx-xapi-pp-cli system-status system-database-information` — Invoke function SystemDatabaseInformation
- `3cx-xapi-pp-cli system-status system-extensions` — Invoke function SystemExtensions
- `3cx-xapi-pp-cli system-status system-health-status` — Invoke function SystemHealthStatus
- `3cx-xapi-pp-cli system-status system-telemetry` — Invoke function SystemTelemetry

**tenant-properties** — Manage tenant properties

- `3cx-xapi-pp-cli tenant-properties create-property` — Add new entity to TenantProperties
- `3cx-xapi-pp-cli tenant-properties list-property` — Get entities from TenantProperties

**tenant-properties-name** — Manage tenant properties name

- `3cx-xapi-pp-cli tenant-properties-name delete-property` — Delete entity from TenantProperties
- `3cx-xapi-pp-cli tenant-properties-name get-property` — Get entity from TenantProperties by key
- `3cx-xapi-pp-cli tenant-properties-name update-property` — Update entity in TenantProperties

**trunk-templates** — Manage trunk templates

- `3cx-xapi-pp-cli trunk-templates create` — Add new entity to TrunkTemplates
- `3cx-xapi-pp-cli trunk-templates list` — Get entities from TrunkTemplates

**trunk-templates-id** — Manage trunk templates id

- `3cx-xapi-pp-cli trunk-templates-id delete-trunk-template` — Delete entity from TrunkTemplates
- `3cx-xapi-pp-cli trunk-templates-id get-trunk-template` — Get entity from TrunkTemplates by key
- `3cx-xapi-pp-cli trunk-templates-id update-trunk-template` — Update entity in TrunkTemplates

**trunks** — Manage trunks

- `3cx-xapi-pp-cli trunks create` — Add new entity to Trunks
- `3cx-xapi-pp-cli trunks finalize-provisioning` — Invoke action FinalizeTrunkProvisioning
- `3cx-xapi-pp-cli trunks get-first-available-number` — Invoke function GetFirstAvailableTrunkNumber
- `3cx-xapi-pp-cli trunks get-provider-phones` — Invoke action GetProviderPhones
- `3cx-xapi-pp-cli trunks init` — Invoke function InitTrunk
- `3cx-xapi-pp-cli trunks init-master-bridge` — Invoke function InitMasterBridge
- `3cx-xapi-pp-cli trunks init-slave-bridge` — Invoke function InitSlaveBridge
- `3cx-xapi-pp-cli trunks list` — Get entities from Trunks
- `3cx-xapi-pp-cli trunks refresh-registration` — Invoke action RefreshRegistration
- `3cx-xapi-pp-cli trunks set-routes` — Invoke action SetRoutes

**trunks-id** — Manage trunks id

- `3cx-xapi-pp-cli trunks-id delete-trunk` — Delete entity from Trunks
- `3cx-xapi-pp-cli trunks-id export-trunk` — Invoke function ExportTrunk
- `3cx-xapi-pp-cli trunks-id get-trunk` — Get entity from Trunks by key
- `3cx-xapi-pp-cli trunks-id run-analysis` — Invoke action RunAnalysis
- `3cx-xapi-pp-cli trunks-id test-inbound-call` — Invoke action TestInboundCall
- `3cx-xapi-pp-cli trunks-id test-outbound-call` — Invoke action TestOutboundCall
- `3cx-xapi-pp-cli trunks-id update-trunk` — Update entity in Trunks

**trunks-number-number** — Manage trunks number number

- `3cx-xapi-pp-cli trunks-number-number <Number>` — Get entity from Trunks by key (Number)

**upgrade-debian** — Manage upgrade debian

- `3cx-xapi-pp-cli upgrade-debian` — Invoke actionImport UpgradeDebian

**users** — Manage users

- `3cx-xapi-pp-cli users batch-delete` — Invoke action BatchDelete
- `3cx-xapi-pp-cli users bulk-update` — Invoke action BulkUpdate
- `3cx-xapi-pp-cli users create` — Add new entity to Users
- `3cx-xapi-pp-cli users download-greeting` — Invoke function DownloadGreeting
- `3cx-xapi-pp-cli users export-extensions` — Invoke function ExportExtensions
- `3cx-xapi-pp-cli users get-duplicated-emails` — Invoke action GetDuplicatedEmails
- `3cx-xapi-pp-cli users get-first-available-extension-number` — Invoke function GetFirstAvailableExtensionNumber
- `3cx-xapi-pp-cli users get-first-available-hotdesking-number` — Invoke function GetFirstAvailableHotdeskingNumber
- `3cx-xapi-pp-cli users get-multi-edit-greetings` — Invoke action GetMultiEditGreetings
- `3cx-xapi-pp-cli users get-phone-registrar` — Invoke function GetPhoneRegistrar
- `3cx-xapi-pp-cli users get-phone-registrars` — Invoke action GetPhoneRegistrars
- `3cx-xapi-pp-cli users install-firmware` — Invoke action InstallFirmware
- `3cx-xapi-pp-cli users list` — Get entities from Users
- `3cx-xapi-pp-cli users make-call` — Invoke action MakeCall
- `3cx-xapi-pp-cli users multi-delete-greeting` — Invoke action MultiDeleteGreeting
- `3cx-xapi-pp-cli users multi-update` — Invoke action MultiUserUpdate
- `3cx-xapi-pp-cli users reboot-phone` — Invoke action RebootPhone
- `3cx-xapi-pp-cli users regenerate-passwords` — Invoke action RegeneratePasswords
- `3cx-xapi-pp-cli users reprovision-all-phones` — Invoke action ReprovisionAllPhones
- `3cx-xapi-pp-cli users reprovision-phone` — Invoke action ReprovisionPhone
- `3cx-xapi-pp-cli users upgrade-phone` — Invoke action UpgradePhone

**users-id** — Manage users id

- `3cx-xapi-pp-cli users-id delete-user` — Delete entity from Users
- `3cx-xapi-pp-cli users-id generate-prov-link` — Invoke function GenerateProvLink
- `3cx-xapi-pp-cli users-id get-phone-secret` — Invoke function GetPhoneSecret
- `3cx-xapi-pp-cli users-id get-user` — Get entity from Users by key
- `3cx-xapi-pp-cli users-id has-duplicated-email` — Invoke function HasDuplicatedEmail
- `3cx-xapi-pp-cli users-id list-forwarding-profiles` — Get ForwardingProfiles from Users
- `3cx-xapi-pp-cli users-id list-greetings` — Get Greetings from Users
- `3cx-xapi-pp-cli users-id list-groups` — Get Groups from Users
- `3cx-xapi-pp-cli users-id make-call-user-record-greeting` — Invoke action MakeCallUserRecordGreeting
- `3cx-xapi-pp-cli users-id regenerate` — Invoke action Regenerate
- `3cx-xapi-pp-cli users-id send-welcome-email` — Invoke action SendWelcomeEmail
- `3cx-xapi-pp-cli users-id set-monitor-status` — Invoke action SetMonitorStatus
- `3cx-xapi-pp-cli users-id update-user` — Update entity in Users

**users-number-number** — Manage users number number

- `3cx-xapi-pp-cli users-number-number <Number>` — Get entity from Users by key (Number)

**voicemail-settings** — Manage voicemail settings

- `3cx-xapi-pp-cli voicemail-settings create-converter-configuration` — Invoke action CreateConverterConfiguration
- `3cx-xapi-pp-cli voicemail-settings delete-all-user-voicemails` — Invoke action DeleteAllUserVoicemails
- `3cx-xapi-pp-cli voicemail-settings delete-converter-configuration` — Invoke action DeleteConverterConfiguration
- `3cx-xapi-pp-cli voicemail-settings get` — Get VoicemailSettings
- `3cx-xapi-pp-cli voicemail-settings get-configured-converters` — Invoke function GetConfiguredConverters
- `3cx-xapi-pp-cli voicemail-settings get-connected-converters-data` — Invoke function GetConnectedConvertersData
- `3cx-xapi-pp-cli voicemail-settings get-converter-request-status` — Invoke function GetConverterRequestStatus
- `3cx-xapi-pp-cli voicemail-settings get-transcribe-languages` — Invoke function GetTranscribeLanguages
- `3cx-xapi-pp-cli voicemail-settings update` — Update VoicemailSettings

**website-links** — Manage website links

- `3cx-xapi-pp-cli website-links bulk-links-delete` — Invoke action BulkLinksDelete
- `3cx-xapi-pp-cli website-links create-weblink` — Add new entity to WebsiteLinks
- `3cx-xapi-pp-cli website-links list-weblink` — Get entities from WebsiteLinks
- `3cx-xapi-pp-cli website-links validate-link` — Invoke action ValidateLink

**website-links-link** — Manage website links link

- `3cx-xapi-pp-cli website-links-link delete-weblink` — Delete entity from WebsiteLinks
- `3cx-xapi-pp-cli website-links-link get-weblink` — Get entity from WebsiteLinks by key
- `3cx-xapi-pp-cli website-links-link update-weblink` — Update entity in WebsiteLinks


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
3cx-xapi-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.


## Recipes

### Audit broken call routing

```bash
3cx-xapi-pp-cli audit --agent
```

After a sync, lists every ring group, queue, rule, and IVR destination that no longer resolves to a live extension or DID.

### Narrow a verbose Users listing for an agent

```bash
3cx-xapi-pp-cli users list --agent --select Number,FirstName,LastName,Groups.Name
```

Users responses are deeply nested; --select with dotted paths returns only the fields an agent needs, saving context.

### Provision a tenant from CSV (safe preview)

```bash
3cx-xapi-pp-cli provision --file new-extensions.csv --dry-run
```

Shows the exact create and update plan with no writes; drop --dry-run to apply idempotently.

### Trace why an extension is not ringing

```bash
3cx-xapi-pp-cli trace 214 --agent
```

Shows every inbound rule, ring group, queue, DID, and forwarding profile that routes to extension 214.

### Weekly queue SLA review

```bash
3cx-xapi-pp-cli qrollup --since 7d --agent
```

One cross-queue table of performance, abandoned calls, and SLA breaches for the week.

## Auth Setup

Authentication is OAuth2 client-credentials. Create an API client in the 3CX Web Client under Admin > Integrations > API, enable the 3CX Configuration API checkbox, and assign the Service Principal the System Owner role (System Administrator causes 403 on call history and several reports). Set TCX_CLIENT_ID and TCX_CLIENT_SECRET, and point the CLI at your instance with TCX_FQDN (or the base-url config). The CLI mints and caches a Bearer token from /connect/token and refreshes it automatically (tokens last 60 minutes).

Run `3cx-xapi-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  3cx-xapi-pp-cli active-calls --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and use `--ignore-missing` only when a missing delete target should count as success

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `API_3CX_XAPI_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `API_3CX_XAPI_CONFIG_DIR`, `API_3CX_XAPI_DATA_DIR`, `API_3CX_XAPI_STATE_DIR`, `API_3CX_XAPI_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `API_3CX_XAPI_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `3cx-xapi-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "3cx-xapi": {
        "command": "3cx-xapi-pp-mcp",
        "env": {
          "API_3CX_XAPI_HOME": "/srv/3cx-xapi"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `API_3CX_XAPI_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `API_3CX_XAPI_HOME`, or `doctor` will not find credentials left under the former root.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
3cx-xapi-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
3cx-xapi-pp-cli feedback --stdin < notes.txt
3cx-xapi-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `API_3CX_XAPI_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `API_3CX_XAPI_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename) |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
3cx-xapi-pp-cli profile save briefing --json
3cx-xapi-pp-cli --profile briefing active-calls
3cx-xapi-pp-cli profile list --json
3cx-xapi-pp-cli profile show briefing
3cx-xapi-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `3cx-xapi-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/3cx-xapi/cmd/3cx-xapi-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add 3cx-xapi-pp-mcp -- 3cx-xapi-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which 3cx-xapi-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   3cx-xapi-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `3cx-xapi-pp-cli <command> --help`.
