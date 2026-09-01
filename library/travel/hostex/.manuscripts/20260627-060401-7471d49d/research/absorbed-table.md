Total operations: 86 (generated as typed commands, grouped by tag). Official MCP server exposes 72 tools over the same surface; our CLI auto-mirrors its Cobra tree to MCP and adds offline SQLite + agent-native output.

| # | Tag | Method | Path | operationId | Our Implementation |
|---|-----|--------|------|-------------|--------------------|
| 1 | Automation | GET | `/automation/actions` | query-automation-actions | (generated endpoint) Automation query-automation-actions |
| 2 | Automation | DELETE | `/automation/actions/{plan_id}` | delete-automation-action | (generated endpoint) Automation delete-automation-action |
| 3 | Automation | POST | `/automation/actions/{plan_id}/execute` | execute-automation-action | (generated endpoint) Automation execute-automation-action |
| 4 | Availability | GET | `/availabilities` | query-availabilities | (generated endpoint) Availability query-availabilities |
| 5 | Availability | POST | `/availabilities` | update-availabilities | (generated endpoint) Availability update-availabilities |
| 6 | Calendar Share Links | GET | `/calendar_share_links` | query-calendar-share-links | (generated endpoint) Calendar Share Links query-calendar-share-links |
| 7 | Calendar Share Links | POST | `/calendar_share_links` | create-calendar-share-link | (generated endpoint) Calendar Share Links create-calendar-share-link |
| 8 | Calendar Share Links | DELETE | `/calendar_share_links/{id}` | delete-calendar-share-link | (generated endpoint) Calendar Share Links delete-calendar-share-link |
| 9 | Channels | GET | `/channel_accounts` | query-channel-accounts | (generated endpoint) Channels query-channel-accounts |
| 10 | Channels | GET | `/listings` | query-listings | (generated endpoint) Channels query-listings |
| 11 | Incomes & Expenses | GET | `/expense_items` | query-expense-items | (generated endpoint) Incomes & Expenses query-expense-items |
| 12 | Incomes & Expenses | GET | `/expense_methods` | query-expense-methods | (generated endpoint) Incomes & Expenses query-expense-methods |
| 13 | Incomes & Expenses | GET | `/income_items` | query-income-items | (generated endpoint) Incomes & Expenses query-income-items |
| 14 | Incomes & Expenses | GET | `/income_methods` | query-income-methods | (generated endpoint) Incomes & Expenses query-income-methods |
| 15 | Incomes & Expenses | GET | `/transactions` | query-transactions | (generated endpoint) Incomes & Expenses query-transactions |
| 16 | Incomes & Expenses | POST | `/transactions` | create-transaction | (generated endpoint) Incomes & Expenses create-transaction |
| 17 | Incomes & Expenses | DELETE | `/transactions/{id}` | delete-transaction | (generated endpoint) Incomes & Expenses delete-transaction |
| 18 | Incomes & Expenses | PATCH | `/transactions/{id}` | update-transaction | (generated endpoint) Incomes & Expenses update-transaction |
| 19 | Knowledge Base | GET | `/knowledge_bases` | query-knowledge-bases | (generated endpoint) Knowledge Base query-knowledge-bases |
| 20 | Knowledge Base | POST | `/knowledge_bases` | create-knowledge-base | (generated endpoint) Knowledge Base create-knowledge-base |
| 21 | Knowledge Base | DELETE | `/knowledge_bases/{id}` | delete-knowledge-base | (generated endpoint) Knowledge Base delete-knowledge-base |
| 22 | Knowledge Base | GET | `/knowledge_bases/{id}` | get-knowledge-base | (generated endpoint) Knowledge Base get-knowledge-base |
| 23 | Knowledge Base | PATCH | `/knowledge_bases/{id}` | update-knowledge-base | (generated endpoint) Knowledge Base update-knowledge-base |
| 24 | Listing Calendar | GET | `/listings/airbnb/price_and_rules` | get-airbnb-listing-price-and-rules | (generated endpoint) Listing Calendar get-airbnb-listing-price-and-rules |
| 25 | Listing Calendar | POST | `/listings/airbnb/price_and_rules` | update-airbnb-listing-price-and-rules | (generated endpoint) Listing Calendar update-airbnb-listing-price-and-rules |
| 26 | Listing Calendar | POST | `/listings/calendar` | query-listing-calendars | (generated endpoint) Listing Calendar query-listing-calendars |
| 27 | Listing Calendar | POST | `/listings/inventories` | update-listing-inventories | (generated endpoint) Listing Calendar update-listing-inventories |
| 28 | Listing Calendar | POST | `/listings/prices` | update-listing-prices | (generated endpoint) Listing Calendar update-listing-prices |
| 29 | Listing Calendar | POST | `/listings/restrictions` | update-listing-restrictions | (generated endpoint) Listing Calendar update-listing-restrictions |
| 30 | Listing Calendar | GET | `/listings/vrbo/price_and_rules` | get-vrbo-listing-price-and-rules | (generated endpoint) Listing Calendar get-vrbo-listing-price-and-rules |
| 31 | Listing Calendar | POST | `/listings/vrbo/price_and_rules` | update-vrbo-listing-price-and-rules | (generated endpoint) Listing Calendar update-vrbo-listing-price-and-rules |
| 32 | Listing Calendar | GET | `/pricing_ratios` | query-pricing-ratios | (generated endpoint) Listing Calendar query-pricing-ratios |
| 33 | Manage Webhooks | GET | `/webhooks` | query-webhooks | (generated endpoint) Manage Webhooks query-webhooks |
| 34 | Manage Webhooks | POST | `/webhooks` | create-webhook | (generated endpoint) Manage Webhooks create-webhook |
| 35 | Manage Webhooks | DELETE | `/webhooks/{id}` | delete-webhook | (generated endpoint) Manage Webhooks delete-webhook |
| 36 | Manage Webhooks | PATCH | `/webhooks/{id}` | update-webhook | (generated endpoint) Manage Webhooks update-webhook |
| 37 | Messages | GET | `/conversations` | query-conversations | (generated endpoint) Messages query-conversations |
| 38 | Messages | GET | `/conversations/{conversation_id}` | get-conversation-details | (generated endpoint) Messages get-conversation-details |
| 39 | Messages | POST | `/conversations/{conversation_id}` | send-message | (generated endpoint) Messages send-message |
| 40 | Messages | PATCH | `/conversations/{conversation_id}/note` | update-conversation-note | (generated endpoint) Messages update-conversation-note |
| 41 | Messages | POST | `/conversations/{conversation_id}/preapprovals` | send-preapproval | (generated endpoint) Messages send-preapproval |
| 42 | Messages | GET | `/conversations/{conversation_id}/special_offers` | get-special-offers | (generated endpoint) Messages get-special-offers |
| 43 | Messages | POST | `/conversations/{conversation_id}/special_offers` | send-special-offer | (generated endpoint) Messages send-special-offer |
| 44 | Messages | DELETE | `/conversations/{conversation_id}/special_offers/{special_offer_id}` | withdraw-special-offer | (generated endpoint) Messages withdraw-special-offer |
| 45 | OAuth | POST | `/oauth/authorizations` | obtain-token | (generated endpoint) OAuth obtain-token |
| 46 | OAuth | POST | `/oauth/revoke` | revoke-token | (generated endpoint) OAuth revoke-token |
| 47 | Property | GET | `/groups` | query-groups | (generated endpoint) Property query-groups |
| 48 | Property | POST | `/groups` | create-group | (generated endpoint) Property create-group |
| 49 | Property | DELETE | `/groups/{id}` | delete-group | (generated endpoint) Property delete-group |
| 50 | Property | PATCH | `/groups/{id}` | update-group | (generated endpoint) Property update-group |
| 51 | Property | GET | `/properties` | query-properties | (generated endpoint) Property query-properties |
| 52 | Property | POST | `/properties` | create-property | (generated endpoint) Property create-property |
| 53 | Property | GET | `/tags` | query-tags | (generated endpoint) Property query-tags |
| 54 | Property | POST | `/tags` | create-tag | (generated endpoint) Property create-tag |
| 55 | Property | DELETE | `/tags/{id}` | delete-tag | (generated endpoint) Property delete-tag |
| 56 | Property | PATCH | `/tags/{id}` | update-tag | (generated endpoint) Property update-tag |
| 57 | Reservation Tags | GET | `/reservation_tags` | query-reservation-tags | (generated endpoint) Reservation Tags query-reservation-tags |
| 58 | Reservation Tags | POST | `/reservation_tags` | create-reservation-tag | (generated endpoint) Reservation Tags create-reservation-tag |
| 59 | Reservation Tags | DELETE | `/reservation_tags/{id}` | delete-reservation-tag | (generated endpoint) Reservation Tags delete-reservation-tag |
| 60 | Reservations | GET | `/custom_channels` | query-custom-channels | (generated endpoint) Reservations query-custom-channels |
| 61 | Reservations | GET | `/reservations` | query-reservations | (generated endpoint) Reservations query-reservations |
| 62 | Reservations | POST | `/reservations` | create-reservation | (generated endpoint) Reservations create-reservation |
| 63 | Reservations | DELETE | `/reservations/{reservation_code}` | cancel-reservation | (generated endpoint) Reservations cancel-reservation |
| 64 | Reservations | POST | `/reservations/{reservation_code}/approve` | approve-reservation | (generated endpoint) Reservations approve-reservation |
| 65 | Reservations | POST | `/reservations/{reservation_code}/decline` | decline-reservation | (generated endpoint) Reservations decline-reservation |
| 66 | Reservations | PATCH | `/reservations/{stay_code}` | update-reservation-basic-info | (generated endpoint) Reservations update-reservation-basic-info |
| 67 | Reservations | POST | `/reservations/{stay_code}/allocate` | allocate-reservation | (generated endpoint) Reservations allocate-reservation |
| 68 | Reservations | PATCH | `/reservations/{stay_code}/check_in_details` | update-check-in-details | (generated endpoint) Reservations update-check-in-details |
| 69 | Reservations | GET | `/reservations/{stay_code}/custom_fields` | query-custom-fields | (generated endpoint) Reservations query-custom-fields |
| 70 | Reservations | PATCH | `/reservations/{stay_code}/custom_fields` | update-custom-fields | (generated endpoint) Reservations update-custom-fields |
| 71 | Reservations | POST | `/reservations/{stay_code}/move_to_box` | move-reservation-to-box | (generated endpoint) Reservations move-reservation-to-box |
| 72 | Reservations | PUT | `/reservations/{stay_code}/stay_status` | update-stay-status | (generated endpoint) Reservations update-stay-status |
| 73 | Reservations | DELETE | `/reservations/{stay_code}/tags` | remove-tag | (generated endpoint) Reservations remove-tag |
| 74 | Reservations | POST | `/reservations/{stay_code}/tags` | add-tag | (generated endpoint) Reservations add-tag |
| 75 | Reviews | GET | `/reviews` | query-reviews | (generated endpoint) Reviews query-reviews |
| 76 | Reviews | POST | `/reviews/{reservation_code}` | create-review | (generated endpoint) Reviews create-review |
| 77 | Room Type | GET | `/room_types` | query-room-types | (generated endpoint) Room Type query-room-types |
| 78 | Room Type | POST | `/room_types` | create-room-type | (generated endpoint) Room Type create-room-type |
| 79 | Task | GET | `/staffs` | query-staffs | (generated endpoint) Task query-staffs |
| 80 | Task | POST | `/staffs` | create-staff | (generated endpoint) Task create-staff |
| 81 | Task | DELETE | `/staffs/{id}` | delete-staff | (generated endpoint) Task delete-staff |
| 82 | Task | PATCH | `/staffs/{id}` | update-staff | (generated endpoint) Task update-staff |
| 83 | Task | GET | `/tasks` | query-tasks | (generated endpoint) Task query-tasks |
| 84 | Task | POST | `/tasks` | create-task | (generated endpoint) Task create-task |
| 85 | Task | DELETE | `/tasks/{id}` | delete-task | (generated endpoint) Task delete-task |
| 86 | Task | PATCH | `/tasks/{id}` | update-task | (generated endpoint) Task update-task |
