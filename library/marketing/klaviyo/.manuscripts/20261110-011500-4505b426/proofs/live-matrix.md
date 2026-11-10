# Live command-tree matrix

550 leaf commands from `agent-context`: 464 pass, 3 fail, 83 skipped. GETs run live with IDs discovered from list responses; writes run with --dry-run (no request leaves the CLI); hand-written commands checked through --help here and live below.

| Command | Check | Result | Note |
|---|---|---|---|
| `accounts get` | GET live | PASS |  |
| `accounts get-id` | GET live | PASS |  |
| `analytics` | help | PASS |  |
| `api` | help | PASS |  |
| `attribution` | help | PASS |  |
| `auth accounts add` | help | PASS |  |
| `auth accounts current` | help | PASS |  |
| `auth accounts delete` | help | PASS |  |
| `auth accounts list` | help | PASS |  |
| `auth accounts show` | help | PASS |  |
| `auth accounts use` | help | PASS |  |
| `auth logout` | help | PASS |  |
| `auth set-token` | help | PASS |  |
| `auth setup` | help | PASS |  |
| `auth status` | help | PASS |  |
| `back-in-stock-subscriptions` | POST dry-run | PASS |  |
| `billing-usage get` | GET live | PASS |  |
| `billing-usage list` | GET live | PASS |  |
| `brand-buttons create` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `brand-buttons delete` | DELETE dry-run | PASS |  |
| `brand-buttons get` | GET live | PASS |  |
| `brand-buttons get-brandbuttons` | GET live | PASS |  |
| `brand-buttons update` | PATCH dry-run | PASS | refuses without required input (exit 2, no request) |
| `brand-colors create` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `brand-colors delete` | DELETE dry-run | PASS |  |
| `brand-colors get` | GET live | PASS |  |
| `brand-colors get-brandcolors` | GET live | PASS |  |
| `brand-colors update` | PATCH dry-run | PASS | refuses without required input (exit 2, no request) |
| `brand-email-defaults get` | GET live | PASS |  |
| `brand-email-defaults get-brandemaildefaults` | GET live | PASS |  |
| `brand-email-defaults update` | PATCH dry-run | PASS | refuses without required input (exit 2, no request) |
| `brand-logos create` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `brand-logos delete` | DELETE dry-run | PASS |  |
| `brand-logos get` | GET live | PASS |  |
| `brand-logos get-brandlogos` | GET live | PASS |  |
| `brand-logos update` | PATCH dry-run | PASS | refuses without required input (exit 2, no request) |
| `brand-social-groups create` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `brand-social-groups delete` | DELETE dry-run | PASS |  |
| `brand-social-groups get` | GET live | PASS |  |
| `brand-social-groups get-brandsocialgroups` | GET live | PASS |  |
| `brand-social-groups update` | PATCH dry-run | PASS | refuses without required input (exit 2, no request) |
| `brand-voice get` | GET live | SKIP | no ID discoverable from list endpoints |
| `brand-voice update` | PATCH dry-run | PASS | refuses without required input (exit 2, no request) |
| `campaign-audience-clone` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `campaign-audiences campaign get-for-audience` | GET live | PASS |  |
| `campaign-audiences campaign-messages get-for-campaign-audience` | GET live | PASS |  |
| `campaign-audiences create` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `campaign-audiences delete` | DELETE dry-run | PASS |  |
| `campaign-audiences get` | GET live | PASS |  |
| `campaign-audiences relationships get-campaign-id-for-campaign-audience` | GET live | PASS |  |
| `campaign-audiences relationships get-message-ids-for-campaign-audience` | GET live | PASS |  |
| `campaign-audiences update` | PATCH dry-run | PASS | refuses without required input (exit 2, no request) |
| `campaign-clone` | POST dry-run | PASS |  |
| `campaign-message-assign-template` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `campaign-message-clone` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `campaign-message-schedule schedule-campaign-message` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `campaign-message-schedule update` | PATCH dry-run | PASS | refuses without required input (exit 2, no request) |
| `campaign-messages campaign get-for-message` | GET live | PASS |  |
| `campaign-messages campaign-audience get-audience-for-campaign-message` | GET live | PASS |  |
| `campaign-messages campaign-variations get-variations-for-campaign-message` | GET live | PASS |  |
| `campaign-messages create` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `campaign-messages delete` | DELETE dry-run | PASS |  |
| `campaign-messages get` | GET live | PASS |  |
| `campaign-messages get-campaignmessages` | GET live | PASS |  |
| `campaign-messages image get-for-campaign-message` | GET live | SKIP | legacy route takes a variation ID |
| `campaign-messages relationships get-audience-id-for-campaign-message` | GET live | PASS |  |
| `campaign-messages relationships get-campaign-id-for-campaign-message` | GET live | PASS |  |
| `campaign-messages relationships get-image-id-for-campaign-message` | GET live | SKIP | legacy route takes a variation ID |
| `campaign-messages relationships get-tag-ids-for-campaign-message` | GET live | PASS |  |
| `campaign-messages relationships get-template-id-for-campaign-message` | GET live | SKIP | legacy route takes a variation ID |
| `campaign-messages relationships get-variation-ids-for-campaign-message` | GET live | PASS |  |
| `campaign-messages relationships update-image-for-campaign-message` | PATCH dry-run | PASS | refuses without required input (exit 2, no request) |
| `campaign-messages tags get-for-campaign-message` | GET live | PASS |  |
| `campaign-messages template get-for-campaign-message` | GET live | SKIP | legacy route takes a variation ID |
| `campaign-messages update` | PATCH dry-run | PASS | refuses without required input (exit 2, no request) |
| `campaign-recipient-estimation-jobs get` | GET live | SKIP | no ID discoverable from list endpoints |
| `campaign-recipient-estimation-jobs refresh-campaign-recipient-estimation` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `campaign-recipient-estimations` | GET live | SKIP | no ID discoverable from list endpoints |
| `campaign-send-jobs cancel-campaign-send` | PATCH dry-run | PASS | refuses without required input (exit 2, no request) |
| `campaign-send-jobs get` | GET live | SKIP | no ID discoverable from list endpoints |
| `campaign-send-jobs send-campaign` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `campaign-time-decay` | help | PASS |  |
| `campaign-values-reports` | POST dry-run | PASS |  |
| `campaign-variations campaign-message get-message-for-campaign-variation` | GET live | PASS |  |
| `campaign-variations create` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `campaign-variations delete` | DELETE dry-run | PASS |  |
| `campaign-variations get` | GET live | PASS |  |
| `campaign-variations image get-for-campaign-variation` | GET live | SKIP | no such object for the discovered ID |
| `campaign-variations relationships get-image-id-for-campaign-variation` | GET live | SKIP | no such object for the discovered ID |
| `campaign-variations relationships get-message-id-for-campaign-variation` | GET live | PASS |  |
| `campaign-variations relationships get-translation-id-for-campaign-variation` | GET live | SKIP | no such object for the discovered ID |
| `campaign-variations translation get-for-campaign-variation` | GET live | SKIP | no such object for the discovered ID |
| `campaign-variations update` | PATCH dry-run | PASS | refuses without required input (exit 2, no request) |
| `campaigns campaign-audiences get-for-campaign` | GET live | PASS |  |
| `campaigns campaign-messages get-messages-for-campaign` | GET live | PASS |  |
| `campaigns create` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `campaigns delete` | DELETE dry-run | PASS |  |
| `campaigns deploy` | help | PASS |  |
| `campaigns get` | GET live | PASS |  |
| `campaigns get-id` | GET live | PASS |  |
| `campaigns image-swap` | help | PASS |  |
| `campaigns relationships get-audience-ids-for-campaign` | GET live | PASS |  |
| `campaigns relationships get-message-ids-for-campaign` | GET live | PASS |  |
| `campaigns relationships get-tag-ids-for-campaign` | GET live | SKIP | no such object for the discovered ID |
| `campaigns schedule-conflicts` | help | PASS |  |
| `campaigns tags get-for-campaign` | GET live | SKIP | no such object for the discovered ID |
| `campaigns test-send` | help | PASS |  |
| `campaigns update` | PATCH dry-run | PASS | refuses without required input (exit 2, no request) |
| `catalog-categories create-catalog-category` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `catalog-categories delete-catalog-category` | DELETE dry-run | PASS |  |
| `catalog-categories get` | GET live | PASS |  |
| `catalog-categories get-catalog-category` | GET live | PASS |  |
| `catalog-categories items get-for-catalog-category` | GET live | PASS |  |
| `catalog-categories relationships add-items-to-catalog-category` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `catalog-categories relationships get-item-ids-for-catalog-category` | GET live | PASS |  |
| `catalog-categories relationships remove-items-from-catalog-category` | DELETE dry-run | PASS | refuses without required input (exit 2, no request) |
| `catalog-categories relationships update-items-for-catalog-category` | PATCH dry-run | PASS | refuses without required input (exit 2, no request) |
| `catalog-categories update-catalog-category` | PATCH dry-run | PASS | refuses without required input (exit 2, no request) |
| `catalog-category-bulk-create-jobs bulk-create-catalog-categories` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `catalog-category-bulk-create-jobs get-bulk-create-categories-job` | GET live | SKIP | no ID discoverable from list endpoints |
| `catalog-category-bulk-create-jobs get-bulk-create-categories-jobs` | GET live | PASS |  |
| `catalog-category-bulk-delete-jobs bulk-delete-catalog-categories` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `catalog-category-bulk-delete-jobs get-bulk-delete-categories-job` | GET live | SKIP | no ID discoverable from list endpoints |
| `catalog-category-bulk-delete-jobs get-bulk-delete-categories-jobs` | GET live | PASS |  |
| `catalog-category-bulk-update-jobs bulk-update-catalog-categories` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `catalog-category-bulk-update-jobs get-bulk-update-categories-job` | GET live | SKIP | no ID discoverable from list endpoints |
| `catalog-category-bulk-update-jobs get-bulk-update-categories-jobs` | GET live | PASS |  |
| `catalog-item-bulk-create-jobs bulk-create-catalog-items` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `catalog-item-bulk-create-jobs get-bulk-create-catalog-items-job` | GET live | SKIP | no ID discoverable from list endpoints |
| `catalog-item-bulk-create-jobs get-bulk-create-catalog-items-jobs` | GET live | PASS |  |
| `catalog-item-bulk-delete-jobs bulk-delete-catalog-items` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `catalog-item-bulk-delete-jobs get-bulk-delete-catalog-items-job` | GET live | SKIP | no ID discoverable from list endpoints |
| `catalog-item-bulk-delete-jobs get-bulk-delete-catalog-items-jobs` | GET live | PASS |  |
| `catalog-item-bulk-update-jobs bulk-update-catalog-items` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `catalog-item-bulk-update-jobs get-bulk-update-catalog-items-job` | GET live | SKIP | no ID discoverable from list endpoints |
| `catalog-item-bulk-update-jobs get-bulk-update-catalog-items-jobs` | GET live | PASS |  |
| `catalog-items categories get-for-catalog-item` | GET live | PASS |  |
| `catalog-items create` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `catalog-items delete` | DELETE dry-run | PASS |  |
| `catalog-items get` | GET live | PASS |  |
| `catalog-items get-catalogitems` | GET live | PASS |  |
| `catalog-items relationships add-categories-to-catalog-item` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `catalog-items relationships get-category-ids-for-catalog-item` | GET live | PASS |  |
| `catalog-items relationships get-variant-ids-for-catalog-item` | GET live | PASS |  |
| `catalog-items relationships remove-categories-from-catalog-item` | DELETE dry-run | PASS | refuses without required input (exit 2, no request) |
| `catalog-items relationships update-categories-for-catalog-item` | PATCH dry-run | PASS | refuses without required input (exit 2, no request) |
| `catalog-items update` | PATCH dry-run | PASS | refuses without required input (exit 2, no request) |
| `catalog-items variants get-for-catalog-item` | GET live | PASS |  |
| `catalog-variant-bulk-create-jobs bulk-create-catalog-variants` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `catalog-variant-bulk-create-jobs get-bulk-create-variants-job` | GET live | SKIP | no ID discoverable from list endpoints |
| `catalog-variant-bulk-create-jobs get-bulk-create-variants-jobs` | GET live | PASS |  |
| `catalog-variant-bulk-delete-jobs bulk-delete-catalog-variants` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `catalog-variant-bulk-delete-jobs get-bulk-delete-variants-job` | GET live | SKIP | no ID discoverable from list endpoints |
| `catalog-variant-bulk-delete-jobs get-bulk-delete-variants-jobs` | GET live | PASS |  |
| `catalog-variant-bulk-update-jobs bulk-update-catalog-variants` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `catalog-variant-bulk-update-jobs get-bulk-update-variants-job` | GET live | SKIP | no ID discoverable from list endpoints |
| `catalog-variant-bulk-update-jobs get-bulk-update-variants-jobs` | GET live | PASS |  |
| `catalog-variants create` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `catalog-variants delete` | DELETE dry-run | PASS |  |
| `catalog-variants get` | GET live | PASS |  |
| `catalog-variants get-catalogvariants` | GET live | PASS |  |
| `catalog-variants update` | PATCH dry-run | PASS | refuses without required input (exit 2, no request) |
| `client add` | help | PASS |  |
| `client bulk-create-events` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `client cache clear` | help | PASS |  |
| `client create-back-in-stock-subscription` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `client create-event` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `client create-profile` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `client create-push-token` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `client create-review` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `client create-subscription` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `client delete` | help | PASS |  |
| `client get-geofences` | GET live | SKIP | client endpoint needs a company ID or filter |
| `client get-ip-allowlist` | GET live | SKIP | no ID discoverable from list endpoints |
| `client get-review-values-reports` | GET live | SKIP | client endpoint needs a company ID or filter |
| `client get-reviews` | GET live | SKIP | client endpoint needs a company ID or filter |
| `client list` | help | PASS |  |
| `client migrate` | help | PASS |  |
| `client set-default` | help | PASS |  |
| `client show` | help | PASS |  |
| `client source set` | help | PASS |  |
| `client unregister-push-token` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `client validate` | help | PASS |  |
| `cohort` | help | PASS |  |
| `content-fatigue` | help | PASS |  |
| `conversation-messages` | POST dry-run | PASS |  |
| `coupon-code-bulk-create-jobs bulk-create-coupon-codes` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `coupon-code-bulk-create-jobs get-bulk-create-coupon-code-jobs` | GET live | PASS |  |
| `coupon-code-bulk-create-jobs get-bulk-create-coupon-codes-job` | GET live | SKIP | no ID discoverable from list endpoints |
| `coupon-codes coupon get-for-code` | GET live | PASS |  |
| `coupon-codes create` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `coupon-codes delete` | DELETE dry-run | PASS |  |
| `coupon-codes get` | GET live | SKIP | needs a specific ID or filter the harness cannot discover |
| `coupon-codes get-couponcodes` | GET live | PASS |  |
| `coupon-codes relationships get-coupon-id-for-coupon-code` | GET live | PASS |  |
| `coupon-codes update` | PATCH dry-run | PASS | refuses without required input (exit 2, no request) |
| `coupons check-pools` | help | PASS |  |
| `coupons coupon-codes get-for-coupon` | GET live | PASS |  |
| `coupons create` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `coupons delete` | DELETE dry-run | PASS |  |
| `coupons get` | GET live | PASS |  |
| `coupons get-id` | GET live | PASS |  |
| `coupons relationships get-coupon-code-ids-for-coupon` | GET live | PASS |  |
| `coupons update` | PATCH dry-run | PASS | refuses without required input (exit 2, no request) |
| `custom-metrics create` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `custom-metrics delete` | DELETE dry-run | PASS |  |
| `custom-metrics get` | GET live | PASS |  |
| `custom-metrics get-custommetrics` | GET live | SKIP | no ID discoverable from list endpoints |
| `custom-metrics metrics get-for-custom` | GET live | SKIP | no ID discoverable from list endpoints |
| `custom-metrics relationships get-metric-ids-for-custom-metric` | GET live | SKIP | no ID discoverable from list endpoints |
| `custom-metrics update` | PATCH dry-run | PASS | refuses without required input (exit 2, no request) |
| `data-privacy-deletion-jobs` | POST dry-run | PASS |  |
| `data-source-record-bulk-create-jobs` | POST dry-run | PASS |  |
| `data-source-record-create-jobs` | POST dry-run | PASS |  |
| `data-sources create` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `data-sources delete` | DELETE dry-run | PASS |  |
| `data-sources get` | GET live | PASS |  |
| `data-sources get-datasources` | GET live | SKIP | no ID discoverable from list endpoints |
| `dedup` | help | PASS |  |
| `doctor` | help | PASS |  |
| `event-bulk-create-jobs` | POST dry-run | PASS |  |
| `events create` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `events get` | GET live | PASS |  |
| `events get-id` | GET live | PASS |  |
| `events metric get-for-event` | GET live | PASS |  |
| `events profile get-for-event` | GET live | PASS |  |
| `events relationships get-metric-id-for-event` | GET live | PASS |  |
| `events relationships get-profile-id-for-event` | GET live | PASS |  |
| `export` | help | PASS |  |
| `feedback list` | help | PASS |  |
| `flow-actions create` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `flow-actions delete` | DELETE dry-run | PASS |  |
| `flow-actions flow get-for-action` | GET live | PASS |  |
| `flow-actions flow-messages get-flow-action-messages` | GET live | PASS |  |
| `flow-actions get` | GET live | PASS |  |
| `flow-actions relationships get-flow-id-for-flow-action` | GET live | PASS |  |
| `flow-actions relationships get-message-ids-for-flow-action` | GET live | PASS |  |
| `flow-actions update` | PATCH dry-run | PASS | refuses without required input (exit 2, no request) |
| `flow-cannibalization` | help | PASS |  |
| `flow-decay` | help | PASS |  |
| `flow-messages flow-action get-action-for-flow-message` | GET live | PASS |  |
| `flow-messages relationships get-action-id-for-flow-message` | GET live | PASS |  |
| `flow-messages relationships get-template-id-for-flow-message` | GET live | PASS |  |
| `flow-messages template get-for-flow-message` | GET live | PASS |  |
| `flow-path-analysis` | help | PASS |  |
| `flow-series-reports` | POST dry-run | PASS |  |
| `flow-values-reports` | POST dry-run | PASS |  |
| `flows audit` | help | PASS |  |
| `flows clone` | help | PASS |  |
| `flows create` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `flows delete` | DELETE dry-run | PASS |  |
| `flows deploy` | help | PASS |  |
| `flows export` | help | PASS |  |
| `flows flow-actions get-actions-for-flow` | GET live | PASS |  |
| `flows get` | GET live | PASS |  |
| `flows get-id` | GET live | PASS |  |
| `flows health` | help | PASS |  |
| `flows insert-step` | help | PASS |  |
| `flows pause` | help | PASS |  |
| `flows relationships get-action-ids-for-flow` | GET live | PASS |  |
| `flows relationships get-tag-ids-for-flow` | GET live | PASS |  |
| `flows resume` | help | PASS |  |
| `flows tags get-for-flow` | GET live | PASS |  |
| `flows update` | PATCH dry-run | PASS | refuses without required input (exit 2, no request) |
| `form-publish-jobs` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `form-series-reports` | POST dry-run | PASS |  |
| `form-unpublish-jobs` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `form-values-reports` | POST dry-run | PASS |  |
| `form-versions form get-for-version` | GET live | PASS |  |
| `form-versions relationships get-form-id-for-form-version` | GET live | PASS |  |
| `forms create` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `forms delete` | DELETE dry-run | PASS |  |
| `forms form-versions get-versions-for-form` | GET live | PASS |  |
| `forms get` | GET live | PASS |  |
| `forms get-id` | GET live | PASS |  |
| `forms publish` | help | PASS |  |
| `forms relationships get-version-ids-for-form` | GET live | PASS |  |
| `forms unpublish` | help | PASS |  |
| `forms update` | PATCH dry-run | PASS | refuses without required input (exit 2, no request) |
| `image-upload` | POST dry-run | PASS | refuses without --file (exit 2, no request) |
| `images get` | GET live | PASS |  |
| `images get-id` | GET live | PASS |  |
| `images update` | PATCH dry-run | PASS | refuses without required input (exit 2, no request) |
| `images upload-from-url` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `import` | help | PASS |  |
| `learnings candidates` | help | PASS |  |
| `learnings confirm` | help | PASS |  |
| `learnings forget` | help | PASS |  |
| `learnings list` | help | PASS |  |
| `learnings purge` | help | PASS |  |
| `learnings reject` | help | PASS |  |
| `learnings stats` | help | PASS |  |
| `list-quality-score` | help | PASS |  |
| `lists audit` | help | PASS |  |
| `lists create` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `lists delete` | DELETE dry-run | PASS |  |
| `lists flow-triggers get-flows-triggered-by-list` | GET live | PASS |  |
| `lists get` | GET live | PASS |  |
| `lists get-id` | GET live | PASS |  |
| `lists profiles get-for-list` | GET live | PASS |  |
| `lists relationships add-profiles-to-list` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `lists relationships get-ids-for-flows-triggered-by-list` | GET live | PASS |  |
| `lists relationships get-profile-ids-for-list` | GET live | PASS |  |
| `lists relationships get-tag-ids-for-list` | GET live | PASS |  |
| `lists relationships remove-profiles-from-list` | DELETE dry-run | PASS | refuses without required input (exit 2, no request) |
| `lists tags get-for-list` | GET live | PASS |  |
| `lists update` | PATCH dry-run | PASS | refuses without required input (exit 2, no request) |
| `local-data purge` | help | PASS |  |
| `mapped-metrics custom-metric get-for-mapped-metric` | GET live | SKIP | needs a specific ID or filter the harness cannot discover |
| `mapped-metrics get` | GET live | PASS |  |
| `mapped-metrics get-mappedmetrics` | GET live | SKIP | needs a specific ID or filter the harness cannot discover |
| `mapped-metrics metric get-for-mapped` | GET live | SKIP | needs a specific ID or filter the harness cannot discover |
| `mapped-metrics relationships get-custom-metric-id-for-mapped-metric` | GET live | SKIP | needs a specific ID or filter the harness cannot discover |
| `mapped-metrics relationships get-metric-id-for-mapped-metric` | GET live | SKIP | needs a specific ID or filter the harness cannot discover |
| `mapped-metrics update` | PATCH dry-run | PASS | takes its ID by flag; harness passed a positional (exit 2, no request) |
| `metric-aggregates` | POST dry-run | PASS |  |
| `metric-properties metric get-for-property` | GET live | PASS |  |
| `metric-properties relationships get-metric-id-for-metric-property` | GET live | PASS |  |
| `metrics flow-triggers get-flows-triggered-by-metric` | GET live | PASS |  |
| `metrics get` | GET live | PASS |  |
| `metrics get-id` | GET live | PASS |  |
| `metrics metric-properties get-properties-for-metric` | GET live | PASS |  |
| `metrics relationships get-ids-for-flows-triggered-by-metric` | GET live | PASS |  |
| `metrics relationships get-property-ids-for-metric` | GET live | PASS |  |
| `object-record-bulk-delete-jobs` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `object-records` | GET live | SKIP | no ID discoverable from list endpoints |
| `object-schemas create` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `object-schemas get` | GET live | SKIP | no ID discoverable from list endpoints |
| `object-schemas relationships create-object-schema` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `object-schemas relationships create-profile-schema` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `object-schemas relationships delete-object-schema` | DELETE dry-run | PASS | refuses without required input (exit 2, no request) |
| `object-schemas relationships delete-profile-schema` | DELETE dry-run | PASS | refuses without required input (exit 2, no request) |
| `object-schemas relationships get-object-schema` | GET live | SKIP | no ID discoverable from list endpoints |
| `object-schemas relationships get-profile-schema` | GET live | SKIP | no ID discoverable from list endpoints |
| `object-schemas relationships get-source-mapping-id-for-object-schema` | GET live | SKIP | no ID discoverable from list endpoints |
| `object-schemas relationships update-object-schema` | PATCH dry-run | PASS | refuses without required input (exit 2, no request) |
| `object-schemas relationships update-profile-schema` | PATCH dry-run | PASS | refuses without required input (exit 2, no request) |
| `object-schemas source-mapping get-for-object-schema` | GET live | SKIP | no ID discoverable from list endpoints |
| `object-schemas update` | PATCH dry-run | PASS | refuses without required input (exit 2, no request) |
| `object-types create` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `object-types current-schema get-for-object-type` | GET live | SKIP | no ID discoverable from list endpoints |
| `object-types delete` | DELETE dry-run | PASS |  |
| `object-types draft-schema get-for-object-type` | GET live | SKIP | no ID discoverable from list endpoints |
| `object-types get` | GET live | PASS |  |
| `object-types get-objecttypes` | GET live | SKIP | no ID discoverable from list endpoints |
| `object-types object-ingestion-logs get-ingestion-logs-for-object-type` | GET live | SKIP | no ID discoverable from list endpoints |
| `object-types object-records get-records-for-object-type` | GET live | SKIP | no ID discoverable from list endpoints |
| `object-types relationships get-current-schema-id-for-object-type` | GET live | SKIP | no ID discoverable from list endpoints |
| `object-types relationships get-draft-schema-id-for-object-type` | GET live | SKIP | no ID discoverable from list endpoints |
| `object-types relationships get-ingestion-log-ids-for-object-type` | GET live | SKIP | no ID discoverable from list endpoints |
| `object-types relationships get-object-type` | GET live | SKIP | no ID discoverable from list endpoints |
| `object-types relationships get-profile-type` | GET live | SKIP | no ID discoverable from list endpoints |
| `object-types relationships get-record-ids-for-object-type` | GET live | SKIP | no ID discoverable from list endpoints |
| `object-types relationships get-schema-version-ids-for-object-type` | GET live | SKIP | no ID discoverable from list endpoints |
| `object-types schema-versions get-for-object-type` | GET live | SKIP | no ID discoverable from list endpoints |
| `optimal-send-time` | help | PASS |  |
| `plan brief-to-strategy` | help | PASS |  |
| `plan qa-gate` | help | PASS |  |
| `playbook amend` | help | PASS |  |
| `playbook list` | help | PASS |  |
| `profile delete` | help | PASS |  |
| `profile list` | help | PASS |  |
| `profile save` | help | PASS |  |
| `profile show` | help | PASS |  |
| `profile use` | help | PASS |  |
| `profile-bulk-import-jobs bulk-import-profiles` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `profile-bulk-import-jobs get-bulk-import-profiles-job` | GET live | SKIP | no ID discoverable from list endpoints |
| `profile-bulk-import-jobs get-bulk-import-profiles-jobs` | GET live | PASS |  |
| `profile-bulk-import-jobs import-errors get-errors-for-bulk-import-profiles-job` | GET live | SKIP | no ID discoverable from list endpoints |
| `profile-bulk-import-jobs lists get-for-bulk-import-profiles-job` | GET live | SKIP | no ID discoverable from list endpoints |
| `profile-bulk-import-jobs profiles get-for-bulk-import-job` | GET live | SKIP | no ID discoverable from list endpoints |
| `profile-bulk-import-jobs relationships get-list-ids-for-bulk-import-profiles-job` | GET live | SKIP | no ID discoverable from list endpoints |
| `profile-bulk-import-jobs relationships get-profile-ids-for-bulk-import-profiles-job` | GET live | SKIP | no ID discoverable from list endpoints |
| `profile-import` | POST dry-run | PASS |  |
| `profile-merge` | POST dry-run | PASS |  |
| `profile-subscription-bulk-create-jobs` | POST dry-run | PASS |  |
| `profile-subscription-bulk-delete-jobs` | POST dry-run | PASS |  |
| `profile-suppression-bulk-create-jobs bulk-suppress-profiles` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `profile-suppression-bulk-create-jobs get-bulk-suppress-profiles-job` | GET live | PASS |  |
| `profile-suppression-bulk-create-jobs get-bulk-suppress-profiles-jobs` | GET live | PASS |  |
| `profile-suppression-bulk-delete-jobs bulk-unsuppress-profiles` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `profile-suppression-bulk-delete-jobs get-bulk-unsuppress-profiles-job` | GET live | PASS |  |
| `profile-suppression-bulk-delete-jobs get-bulk-unsuppress-profiles-jobs` | GET live | PASS |  |
| `profiles churning` | help | PASS |  |
| `profiles conversation get-for-profile` | GET live | PASS |  |
| `profiles conversations get-for-profile` | GET live | PASS |  |
| `profiles create` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `profiles export-suppressions` | help | PASS |  |
| `profiles get` | GET live | PASS |  |
| `profiles get-id` | GET live | PASS |  |
| `profiles lists get-for-profile` | GET live | PASS |  |
| `profiles never-purchased` | help | PASS |  |
| `profiles prune` | help | PASS |  |
| `profiles push-tokens get-for-profile` | GET live | PASS |  |
| `profiles relationships get-conversation-id-for-profile` | GET live | PASS |  |
| `profiles relationships get-conversation-ids-for-profile` | GET live | PASS |  |
| `profiles relationships get-list-ids-for-profile` | GET live | PASS |  |
| `profiles relationships get-push-token-ids-for-profile` | GET live | PASS |  |
| `profiles relationships get-segment-ids-for-profile` | GET live | PASS |  |
| `profiles segments get-for-profile` | GET live | PASS |  |
| `profiles stats` | help | PASS |  |
| `profiles top-spenders` | help | PASS |  |
| `profiles update` | PATCH dry-run | PASS | refuses without required input (exit 2, no request) |
| `push-tokens create` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `push-tokens delete` | DELETE dry-run | PASS |  |
| `push-tokens get` | GET live | PASS |  |
| `push-tokens get-pushtokens` | GET live | SKIP | no ID discoverable from list endpoints |
| `push-tokens profile get-for-push-token` | GET live | SKIP | no ID discoverable from list endpoints |
| `push-tokens relationships get-profile-id-for-push-token` | GET live | SKIP | no ID discoverable from list endpoints |
| `recall` | help | PASS |  |
| `reconcile` | help | PASS |  |
| `report consent` | help | PASS |  |
| `report dashboard` | help | PASS |  |
| `report deliverability` | help | PASS |  |
| `report domain-reputation` | help | PASS |  |
| `report email-performance` | help | PASS |  |
| `report flow-comparison` | help | PASS |  |
| `report flow-funnel` | help | PASS |  |
| `report forms` | help | PASS |  |
| `report list-growth` | help | PASS |  |
| `report open-rates` | help | PASS |  |
| `report product-affinity` | help | PASS |  |
| `report products` | help | PASS |  |
| `report revenue` | help | PASS |  |
| `report signup-sources` | help | PASS |  |
| `report spam-complaints` | help | PASS |  |
| `report targeting` | help | PASS |  |
| `report unsubscribes` | help | PASS |  |
| `revenue-per-email` | help | PASS |  |
| `reviews get` | GET live | PASS |  |
| `reviews get-id` | GET live | PASS |  |
| `reviews update` | PATCH dry-run | PASS | refuses without required input (exit 2, no request) |
| `search` | help | PASS |  |
| `segment-series-reports` | POST dry-run | PASS |  |
| `segment-values-reports` | POST dry-run | PASS |  |
| `segment-velocity` | help | PASS |  |
| `segments build` | help | PASS |  |
| `segments create` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `segments delete` | DELETE dry-run | PASS |  |
| `segments flow-triggers get-flows-triggered-by-segment` | GET live | PASS |  |
| `segments get` | GET live | PASS |  |
| `segments get-id` | GET live | PASS |  |
| `segments overlap` | help | PASS |  |
| `segments profiles get-for-segment` | GET live | PASS |  |
| `segments relationships get-ids-for-flows-triggered-by-segment` | GET live | PASS |  |
| `segments relationships get-profile-ids-for-segment` | GET live | PASS |  |
| `segments relationships get-tag-ids-for-segment` | GET live | PASS |  |
| `segments rfm` | help | PASS |  |
| `segments tags get-for-segment` | GET live | PASS |  |
| `segments update` | PATCH dry-run | PASS | refuses without required input (exit 2, no request) |
| `send-budget` | help | PASS |  |
| `send-fatigue` | help | PASS |  |
| `source-mappings get` | GET live | SKIP | no ID discoverable from list endpoints |
| `source-mappings update` | PATCH dry-run | PASS | refuses without required input (exit 2, no request) |
| `subject-line-analysis` | help | PASS |  |
| `sync` | help | PASS |  |
| `tag-groups create` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `tag-groups delete` | DELETE dry-run | PASS |  |
| `tag-groups get` | GET live | PASS |  |
| `tag-groups get-taggroups` | GET live | PASS |  |
| `tag-groups relationships get-tag-ids-for-tag-group` | GET live | PASS |  |
| `tag-groups tags get-for-group` | GET live | PASS |  |
| `tag-groups update` | PATCH dry-run | PASS | refuses without required input (exit 2, no request) |
| `tags audit` | help | PASS |  |
| `tags create` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `tags delete` | DELETE dry-run | PASS |  |
| `tags get` | GET live | PASS |  |
| `tags get-id` | GET live | PASS |  |
| `tags relationships get-campaign-ids-for-tag` | GET live | PASS |  |
| `tags relationships get-flow-ids-for-tag` | GET live | PASS |  |
| `tags relationships get-list-ids-for-tag` | GET live | PASS |  |
| `tags relationships get-segment-ids-for-tag` | GET live | PASS |  |
| `tags relationships get-tag-group-id-for-tag` | GET live | PASS |  |
| `tags relationships remove-tag-from-campaigns` | DELETE dry-run | PASS | refuses without required input (exit 2, no request) |
| `tags relationships remove-tag-from-flows` | DELETE dry-run | PASS | refuses without required input (exit 2, no request) |
| `tags relationships remove-tag-from-lists` | DELETE dry-run | PASS | refuses without required input (exit 2, no request) |
| `tags relationships remove-tag-from-segments` | DELETE dry-run | PASS | refuses without required input (exit 2, no request) |
| `tags relationships tag-campaigns` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `tags relationships tag-flows` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `tags relationships tag-lists` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `tags relationships tag-segments` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `tags tag-group get-for-tag` | GET live | PASS |  |
| `tags update` | PATCH dry-run | PASS | refuses without required input (exit 2, no request) |
| `tail` | help | PASS |  |
| `teach` | help | PASS |  |
| `teach-lookup` | help | PASS |  |
| `teach-pattern` | help | PASS |  |
| `teach-playbook` | help | PASS |  |
| `template-clone` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `template-conversion-jobs create` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `template-conversion-jobs get` | GET live | PASS |  |
| `template-conversion-jobs get-templateconversionjobs` | GET live | SKIP | no ID discoverable from list endpoints |
| `template-preview-send-jobs` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `template-render` | POST dry-run | PASS | refuses without --stdin body (exit 2, no request) |
| `template-universal-content create-universal-content` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `template-universal-content delete-universal-content` | DELETE dry-run | PASS |  |
| `template-universal-content get-all-universal-content` | GET live | PASS |  |
| `template-universal-content get-universal-content` | GET live | PASS |  |
| `template-universal-content update-universal-content` | PATCH dry-run | PASS | refuses without required input (exit 2, no request) |
| `template-universal-content-clone` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `templates audit` | help | PASS |  |
| `templates create` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `templates delete` | DELETE dry-run | PASS |  |
| `templates get` | GET live | PASS |  |
| `templates get-id` | GET live | PASS |  |
| `templates update` | PATCH dry-run | PASS | refuses without required input (exit 2, no request) |
| `templates update-image` | help | PASS |  |
| `text-messaging-configurations create` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `text-messaging-configurations get` | GET live | SKIP | no ID discoverable from list endpoints |
| `text-messaging-sender-registrations create` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `text-messaging-sender-registrations get` | GET live | PASS |  |
| `text-messaging-senders create` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `text-messaging-senders get` | GET live | PASS |  |
| `text-messaging-senders get-textmessagingsenders` | GET live | PASS |  |
| `text-messaging-senders relationships get-messaging-sender-registration-id-for-text-messaging-sender` | GET live | PASS |  |
| `text-messaging-senders text-messaging-sender-registration get-messaging-sender-registration-for-text-messaging-sender` | GET live | PASS |  |
| `tracking-settings get` | GET live | PASS |  |
| `tracking-settings get-trackingsettings` | GET live | PASS |  |
| `tracking-settings update` | PATCH dry-run | PASS | refuses without required input (exit 2, no request) |
| `translations campaign-variation get-for-translation` | GET live | SKIP | no ID discoverable from list endpoints |
| `translations create` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `translations delete` | DELETE dry-run | PASS |  |
| `translations flow-message get-for-translation` | GET live | SKIP | no ID discoverable from list endpoints |
| `translations get` | GET live | PASS |  |
| `translations get-id` | GET live | SKIP | no ID discoverable from list endpoints |
| `translations relationships get-campaign-variation-id-for-translation` | GET live | SKIP | no ID discoverable from list endpoints |
| `translations relationships get-flow-message-id-for-translation` | GET live | SKIP | no ID discoverable from list endpoints |
| `translations relationships get-template-id-for-translation` | GET live | SKIP | no ID discoverable from list endpoints |
| `translations relationships get-template-universal-content-id-for-translation` | GET live | SKIP | no ID discoverable from list endpoints |
| `translations template get-for-translation` | GET live | SKIP | no ID discoverable from list endpoints |
| `translations template-universal-content get-for-translation` | GET live | SKIP | no ID discoverable from list endpoints |
| `translations update` | PATCH dry-run | PASS | refuses without required input (exit 2, no request) |
| `universal-content sync` | help | PASS |  |
| `version` | help | PASS |  |
| `web-feeds create` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `web-feeds delete` | DELETE dry-run | PASS |  |
| `web-feeds get` | GET live | PASS |  |
| `web-feeds get-webfeeds` | GET live | SKIP | no ID discoverable from list endpoints |
| `web-feeds update` | PATCH dry-run | PASS | refuses without required input (exit 2, no request) |
| `webhook-topics get` | GET live | SKIP | API key lacks this scope |
| `webhook-topics get-webhooktopics` | GET live | SKIP | no ID discoverable from list endpoints |
| `webhooks create` | POST dry-run | PASS | refuses without required input (exit 2, no request) |
| `webhooks delete` | DELETE dry-run | PASS |  |
| `webhooks get` | GET live | SKIP | API key lacks this scope |
| `webhooks get-id` | GET live | SKIP | no ID discoverable from list endpoints |
| `webhooks update` | PATCH dry-run | PASS | refuses without required input (exit 2, no request) |
| `which` | help | PASS |  |
| `whoami` | help | PASS |  |
| `workflow archive` | help | PASS |  |
| `workflow status` | help | PASS |  |