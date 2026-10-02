# Loops command effects

This matrix describes every runnable command in the Printing Press 4.33.0 print of Loops OpenAPI 1.22.2. It is generated from the CLI command tree; account-specific names, contact values, and IDs are omitted.

**Reads data** never changes Loops. **Changes data** requires the listed flags and a live exact team check. **Sends email** and event sends require a caller-provided idempotency key where Loops supports it. Default mutation invocation is a redacted preview with no API call. `--yes` and `--agent` do not bypass the execution gate.

## API commands

| Command | API | Effect | Live gate |
| --- | --- | --- | --- |
| `loops-pp-cli api-key` | `GET /v1/api-key` | Reads Loops data | None |
| `loops-pp-cli audience-segments create` | `POST /v1/audience-segments` | Changes Loops data | --execute --team |
| `loops-pp-cli audience-segments get` | `GET /v1/audience-segments/{audienceSegmentId}` | Reads Loops data | None |
| `loops-pp-cli audience-segments list` | `GET /v1/audience-segments` | Reads Loops data | None |
| `loops-pp-cli campaign-groups create` | `POST /v1/campaign-groups` | Changes Loops data | --execute --team |
| `loops-pp-cli campaign-groups get` | `GET /v1/campaign-groups/{campaignGroupId}` | Reads Loops data | None |
| `loops-pp-cli campaign-groups list` | `GET /v1/campaign-groups` | Reads Loops data | None |
| `loops-pp-cli campaign-groups update` | `POST /v1/campaign-groups/{campaignGroupId}` | Changes Loops data | --execute --team |
| `loops-pp-cli campaigns create` | `POST /v1/campaigns` | Changes Loops data; may publish or schedule email | --execute --team --confirm-publish |
| `loops-pp-cli campaigns get` | `GET /v1/campaigns/{campaignId}` | Reads Loops data | None |
| `loops-pp-cli campaigns list` | `GET /v1/campaigns` | Reads Loops data | None |
| `loops-pp-cli campaigns metrics get-campaign` | `GET /v1/campaigns/{campaignId}/metrics` | Reads Loops data | None |
| `loops-pp-cli campaigns update` | `POST /v1/campaigns/{campaignId}` | Changes Loops data; may publish or schedule email | --execute --team --confirm-publish |
| `loops-pp-cli components create` | `POST /v1/components` | Changes Loops data | --execute --team |
| `loops-pp-cli components get` | `GET /v1/components/{componentId}` | Reads Loops data | None |
| `loops-pp-cli components list` | `GET /v1/components` | Reads Loops data | None |
| `loops-pp-cli components update` | `POST /v1/components/{componentId}` | Changes Loops data | --execute --team |
| `loops-pp-cli contacts create` | `POST /v1/contacts/create` | Changes Loops data | --execute --team |
| `loops-pp-cli contacts create-property` | `POST /v1/contacts/properties` | Changes Loops data | --execute --team |
| `loops-pp-cli contacts delete` | `POST /v1/contacts/delete` | Deletes Loops data | --execute --team --confirm-destructive |
| `loops-pp-cli contacts find` | `GET /v1/contacts/find` | Reads Loops data | None |
| `loops-pp-cli contacts get-suppression` | `GET /v1/contacts/suppression` | Reads Loops data | None |
| `loops-pp-cli contacts list-properties` | `GET /v1/contacts/properties` | Reads Loops data | None |
| `loops-pp-cli contacts remove-suppression` | `DELETE /v1/contacts/suppression` | Changes Loops data (removes suppression) | --execute --team --confirm-suppression-removal |
| `loops-pp-cli contacts update` | `PUT /v1/contacts/update` | Changes Loops data | --execute --team |
| `loops-pp-cli dedicated-sending-ips` | `GET /v1/dedicated-sending-ips` | Reads Loops data | None |
| `loops-pp-cli email-messages get` | `GET /v1/email-messages/{emailMessageId}` | Reads Loops data | None |
| `loops-pp-cli email-messages guardian get-email-message` | `GET /v1/email-messages/{emailMessageId}/guardian` | Reads Loops data | None |
| `loops-pp-cli email-messages preview email-message` | `POST /v1/email-messages/{emailMessageId}/preview` | Sends preview email | --execute --team --confirm-send |
| `loops-pp-cli email-messages update` | `POST /v1/email-messages/{emailMessageId}` | Changes Loops data | --execute --team |
| `loops-pp-cli event-patterns get` | `GET /v1/event-patterns/{eventPatternId}` | Reads Loops data | None |
| `loops-pp-cli event-patterns get-by-name` | `GET /v1/event-patterns/by-name/{eventName}` | Reads Loops data | None |
| `loops-pp-cli event-patterns list` | `GET /v1/event-patterns` | Reads Loops data | None |
| `loops-pp-cli events` | `POST /v1/events/send` | Sends event; may send email | --execute --team --confirm-send --idempotency-key |
| `loops-pp-cli lists` | `GET /v1/lists` | Reads Loops data | None |
| `loops-pp-cli themes create` | `POST /v1/themes` | Changes Loops data | --execute --team |
| `loops-pp-cli themes get` | `GET /v1/themes/{themeId}` | Reads Loops data | None |
| `loops-pp-cli themes list` | `GET /v1/themes` | Reads Loops data | None |
| `loops-pp-cli themes update` | `POST /v1/themes/{themeId}` | Changes Loops data | --execute --team |
| `loops-pp-cli transactional list-published-emails` | `GET /v1/transactional` | Reads Loops data | None |
| `loops-pp-cli transactional send-email` | `POST /v1/transactional` | Sends email | --execute --team --confirm-send --idempotency-key |
| `loops-pp-cli transactional-emails create` | `POST /v1/transactional-emails` | Changes Loops data | --execute --team |
| `loops-pp-cli transactional-emails draft ensure-transactional` | `POST /v1/transactional-emails/{transactionalId}/draft` | Changes Loops data | --execute --team |
| `loops-pp-cli transactional-emails get` | `GET /v1/transactional-emails/{transactionalId}` | Reads Loops data | None |
| `loops-pp-cli transactional-emails list` | `GET /v1/transactional-emails` | Reads Loops data | None |
| `loops-pp-cli transactional-emails metrics get-transactional-email` | `GET /v1/transactional-emails/{transactionalId}/metrics` | Reads Loops data | None |
| `loops-pp-cli transactional-emails publish transactional-email` | `POST /v1/transactional-emails/{transactionalId}/publish` | Changes Loops data; may publish or schedule email | --execute --team --confirm-publish |
| `loops-pp-cli transactional-emails update` | `POST /v1/transactional-emails/{transactionalId}` | Changes Loops data | --execute --team |
| `loops-pp-cli transactional-groups create` | `POST /v1/transactional-groups` | Changes Loops data | --execute --team |
| `loops-pp-cli transactional-groups get` | `GET /v1/transactional-groups/{transactionalGroupId}` | Reads Loops data | None |
| `loops-pp-cli transactional-groups list` | `GET /v1/transactional-groups` | Reads Loops data | None |
| `loops-pp-cli transactional-groups update` | `POST /v1/transactional-groups/{transactionalGroupId}` | Changes Loops data | --execute --team |
| `loops-pp-cli uploads` | `POST /v1/uploads` | Changes Loops data | --execute --team |
| `loops-pp-cli uploads complete upload` | `POST /v1/uploads/{emailAssetId}/complete` | Changes Loops data | --execute --team |
| `loops-pp-cli workflows create` | `POST /v1/workflows` | Changes Loops data | --execute --team |
| `loops-pp-cli workflows delete` | `DELETE /v1/workflows/{workflowId}` | Deletes Loops data | --execute --team --confirm-destructive |
| `loops-pp-cli workflows get` | `GET /v1/workflows/{workflowId}` | Reads Loops data | None |
| `loops-pp-cli workflows list` | `GET /v1/workflows` | Reads Loops data | None |
| `loops-pp-cli workflows mailing-list change-workflow` | `POST /v1/workflows/{workflowId}/mailing-list` | Changes Loops data | --execute --team |
| `loops-pp-cli workflows nodes add-workflow-branch` | `POST /v1/workflows/{workflowId}/nodes/{nodeId}/add-branch` | Changes Loops data | --execute --team |
| `loops-pp-cli workflows nodes create-workflow` | `POST /v1/workflows/{workflowId}/nodes` | Changes Loops data | --execute --team |
| `loops-pp-cli workflows nodes delete-workflow` | `DELETE /v1/workflows/{workflowId}/nodes/{nodeId}` | Deletes Loops data | --execute --team --confirm-destructive |
| `loops-pp-cli workflows nodes delete-workflow-recursively` | `DELETE /v1/workflows/{workflowId}/nodes/{nodeId}/recursive` | Deletes Loops data | --execute --team --confirm-destructive |
| `loops-pp-cli workflows nodes get-workflow` | `GET /v1/workflows/{workflowId}/nodes/{nodeId}` | Reads Loops data | None |
| `loops-pp-cli workflows nodes get-workflow-metrics` | `GET /v1/workflows/{workflowId}/nodes/{nodeId}/metrics` | Reads Loops data | None |
| `loops-pp-cli workflows nodes reroute-connection` | `POST /v1/workflows/{workflowId}/nodes/{nodeId}/reroute` | Changes Loops data | --execute --team |
| `loops-pp-cli workflows nodes update-workflow` | `POST /v1/workflows/{workflowId}/nodes/{nodeId}` | Changes Loops data | --execute --team |
| `loops-pp-cli workflows update-properties` | `POST /v1/workflows/{workflowId}` | Changes Loops data | --execute --team |

## Agent and framework commands

Parent group commands display help and read no account data. Their children and standalone commands have the effects below.

| Command | Effect |
| --- | --- |
| `loops-pp-cli analytics` | Reads local CLI data; no email |
| `loops-pp-cli api` | Reads local CLI configuration or metadata; no email |
| `loops-pp-cli audience-segments` | Reads command help; no API call |
| `loops-pp-cli audit lifecycle` | Reads Loops data; no email |
| `loops-pp-cli auth` | Reads command help; no API call |
| `loops-pp-cli auth logout` | Changes local CLI data; no Loops email |
| `loops-pp-cli auth set-token` | Changes local CLI data; no Loops email |
| `loops-pp-cli auth setup` | Changes local CLI data; no Loops email |
| `loops-pp-cli auth status` | Reads local CLI configuration or metadata; no email |
| `loops-pp-cli campaign-groups` | Reads command help; no API call |
| `loops-pp-cli campaigns` | Reads command help; no API call |
| `loops-pp-cli campaigns metrics` | Reads command help; no API call |
| `loops-pp-cli campaigns preflight` | Reads Loops data; no email |
| `loops-pp-cli completion bash` | Reads command metadata; emits shell script |
| `loops-pp-cli completion fish` | Reads command metadata; emits shell script |
| `loops-pp-cli completion powershell` | Reads command metadata; emits shell script |
| `loops-pp-cli completion zsh` | Reads command metadata; emits shell script |
| `loops-pp-cli components` | Reads command help; no API call |
| `loops-pp-cli contacts` | Reads command help; no API call |
| `loops-pp-cli doctor` | Reads Loops data; no email |
| `loops-pp-cli email-messages` | Reads command help; no API call |
| `loops-pp-cli email-messages guardian` | Reads command help; no API call |
| `loops-pp-cli email-messages preview` | Reads command help; no API call |
| `loops-pp-cli event-patterns` | Reads command help; no API call |
| `loops-pp-cli export` | Reads data; writes local export |
| `loops-pp-cli feedback` | Changes local CLI data; no Loops email |
| `loops-pp-cli feedback list` | Reads local CLI data; no email |
| `loops-pp-cli help` | Reads local CLI configuration or metadata; no email |
| `loops-pp-cli import` | Changes Loops data in bulk; event/email import blocked |
| `loops-pp-cli learnings` | Reads command help; no API call |
| `loops-pp-cli learnings candidates` | Reads local CLI data; no email |
| `loops-pp-cli learnings confirm` | Changes local CLI data; no Loops email |
| `loops-pp-cli learnings forget` | Changes local CLI data; no Loops email |
| `loops-pp-cli learnings list` | Reads local CLI data; no email |
| `loops-pp-cli learnings purge` | Changes local CLI data; no Loops email |
| `loops-pp-cli learnings reject` | Changes local CLI data; no Loops email |
| `loops-pp-cli learnings stats` | Reads local CLI data; no email |
| `loops-pp-cli playbook` | Reads command help; no API call |
| `loops-pp-cli playbook amend` | Changes local CLI data; no Loops email |
| `loops-pp-cli playbook list` | Reads local CLI data; no email |
| `loops-pp-cli profile` | Reads command help; no API call |
| `loops-pp-cli profile delete` | Changes local CLI data; no Loops email |
| `loops-pp-cli profile list` | Reads local CLI data; no email |
| `loops-pp-cli profile save` | Changes local CLI data; no Loops email |
| `loops-pp-cli profile show` | Reads local CLI data; no email |
| `loops-pp-cli profile use` | Changes local CLI data; no Loops email |
| `loops-pp-cli recall` | Reads local CLI data; no email |
| `loops-pp-cli search` | Reads local CLI data; no email |
| `loops-pp-cli sync` | Reads Loops data; changes local cache |
| `loops-pp-cli tail` | Reads Loops data; no email |
| `loops-pp-cli teach` | Changes local CLI data; no Loops email |
| `loops-pp-cli teach-lookup` | Changes local CLI data; no Loops email |
| `loops-pp-cli teach-pattern` | Changes local CLI data; no Loops email |
| `loops-pp-cli teach-playbook` | Changes local CLI data; no Loops email |
| `loops-pp-cli team verify` | Reads Loops data; no email |
| `loops-pp-cli themes` | Reads command help; no API call |
| `loops-pp-cli transactional` | Reads command help; no API call |
| `loops-pp-cli transactional-emails` | Reads command help; no API call |
| `loops-pp-cli transactional-emails draft` | Reads command help; no API call |
| `loops-pp-cli transactional-emails metrics` | Reads command help; no API call |
| `loops-pp-cli transactional-emails publish` | Reads command help; no API call |
| `loops-pp-cli transactional-groups` | Reads command help; no API call |
| `loops-pp-cli uploads complete` | Reads command help; no API call |
| `loops-pp-cli version` | Reads local CLI configuration or metadata; no email |
| `loops-pp-cli which` | Reads local CLI configuration or metadata; no email |
| `loops-pp-cli workflow` | Reads command help; no API call |
| `loops-pp-cli workflow archive` | Reads Loops data; changes local archive |
| `loops-pp-cli workflow status` | Reads local CLI data; no email |
| `loops-pp-cli workflows` | Reads command help; no API call |
| `loops-pp-cli workflows mailing-list` | Reads command help; no API call |
| `loops-pp-cli workflows nodes` | Reads command help; no API call |

Bulk `import` has the same `--execute --team` gate as other writes. It cannot bulk-send events or transactional email because each send needs its own idempotency key. `sync`, `export`, and `tail` exclude contacts: Loops provides contact lookup by identifier, not a bulk contact list. API GET commands may return detailed account records; use `--agent`, `--select`, and `--no-cache` when only a few fields are needed.
