# Salesforce Headless 360 Trust Metadata

This directory ships the custom objects, custom fields, Custom Metadata Type, and permission set the CLI needs in every target org.

## Deploy

From the CLI root (one level up from this directory):

```sh
sf project deploy start --source-dir metadata --target-org <alias>
sf org assign permset --name SF360_Key_Registrar --target-org <alias>
```

A `sfdx-project.json` lives at the CLI root so the deploy command works from a fresh clone with no extra configuration.

After deployment, register the local signing key:

```sh
salesforce-headless-360-pp-cli trust register --org <alias>
```

`trust register` prefers Salesforce `Certificate` records. If the target edition rejects them (`INVALID_TYPE` / `NOT_FOUND`), it falls back to a `SF360_Bundle_Key__mdt` row with a signed hash-chain receipt.

## What gets deployed

### Custom objects

- **`SF360_Bundle_Audit__c`** — append-only audit rows emitted for every signed read bundle. GDPR Article 30 record-of-processing.
- **`SF360_Write_Audit__c`** — signed write-intent audit rows (one per write verb). Holds the JWS, field diff, idempotency key, and execution status.

### Custom fields on standard objects

`SF360_Idempotency_Key__c` is a unique, case-sensitive External ID Text field deployed on:

- Account
- Contact
- Opportunity
- Case

### Custom Metadata Type

- **`SF360_Bundle_Key__mdt`** — fallback store for bundle-signing public keys when the org edition cannot create Certificate records.

### Permission set

- **`SF360_Key_Registrar`** — CRUD on `SF360_Write_Audit__c`, read on `SF360_Bundle_Audit__c`, read/write on `SF360_Bundle_Key__mdt`. Assign to every user the CLI will run as.

## Source tree layout

This directory follows SFDX source format:

```
metadata/
├── objects/
│   ├── Account/fields/SF360_Idempotency_Key__c.field-meta.xml
│   ├── Case/fields/SF360_Idempotency_Key__c.field-meta.xml
│   ├── Contact/fields/SF360_Idempotency_Key__c.field-meta.xml
│   ├── Opportunity/fields/SF360_Idempotency_Key__c.field-meta.xml
│   ├── SF360_Bundle_Audit__c/...
│   ├── SF360_Bundle_Key__mdt/...
│   └── SF360_Write_Audit__c/...
├── permissionsets/
│   └── SF360_Key_Registrar.permissionset-meta.xml
└── README.md
```

All `<fullName>` elements in field XML files are bare (no `ObjectName.` prefix). This is the layout `sf project deploy start --source-dir metadata` expects.

## Known limitations

### Task and Event idempotency fields are not deployed

`SF360_Idempotency_Key__c` would ideally exist on `Task` and `Event` in addition to the four objects listed above. Salesforce's handling of Activity-style objects (Task + Event share the `Activity` parent type) rejects direct External ID Text fields with a restricted-picklist error during deploy.

Status: deferred to v1.2 pending Activity-object research. See `docs/findings/2026-04-24-live-verify-findings.md#finding-f-008` for details.

Impact on write verbs: `agent log-activity` (W3 in live-verify) creates Tasks without persisting an idempotency key. Retries of the same `--idempotency-key` argument may create duplicate Tasks. Agents should treat Task creation as non-idempotent until this is resolved.

### Audit index notes

`TargetRecordId__c` and `IdempotencyKey__c` are declared as External ID Text fields so Salesforce creates the per-field index automatically. `ActingKid__c` and `TraceId__c` remain plain text — org admins can request custom indexes separately if their audit query patterns need them.

## Troubleshooting

**Deploy fails with `TypeInferenceError: Could not infer a metadata type`:**
You cloned a pre-2026-04-24 version. The `metadata/fields/` flat layout is no longer shipped. Pull latest.

**Deploy fails with `Cannot specify: deploymentStatus for Custom Metadata Type`:**
Same — pre-2026-04-24 version. CMDT no longer declares `<deploymentStatus>`. Pull latest.

**Deploy fails with `Cannot add a lookup relationship child with cascade or restrict options to User`:**
Your working copy has the pre-fix ActingUser__c declaration. The lookup is now optional with no delete constraint. `git checkout metadata/objects/SF360_Write_Audit__c/fields/ActingUser__c.field-meta.xml`.

**Deploy fails with `You cannot deploy to a required field`:**
Your permission set declares FLS for required audit fields. Required fields auto-grant FLS — declarations are redundant and rejected. Strip the relevant `<fieldPermissions>` blocks from `SF360_Key_Registrar.permissionset-meta.xml`.

See `docs/findings/2026-04-24-live-verify-findings.md` for the full post-mortem of each of these gotchas.
