# SendFox absorb manifest

## Absorbed
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | List contacts | Official v1.4.0 GET /contacts | (generated endpoint) contacts listContacts | Typed validation, JSON/select, preview and error recovery |
| 2 | Create a new contact | Official v1.4.0 POST /contacts | (generated endpoint) contacts createContact | Typed validation, JSON/select, preview and error recovery |
| 3 | Get a specific contact | Official v1.4.0 GET /contacts/{id} | (generated endpoint) contacts getContact | Typed validation, JSON/select, preview and error recovery |
| 4 | Update a contact | Official v1.4.0 PATCH /contacts/{id} | (generated endpoint) contacts updateContact | Typed validation, JSON/select, preview and error recovery |
| 5 | Delete a contact | Official v1.4.0 DELETE /contacts/{id} | (generated endpoint) contacts deleteContact | Typed validation, JSON/select, preview and error recovery |
| 6 | Get email activity for a contact | Official v1.4.0 GET /contacts/{id}/activity | (generated endpoint) contacts getContactActivity | Typed validation, JSON/select, preview and error recovery |
| 7 | Batch import contacts | Official v1.4.0 POST /contacts/batch | (generated endpoint) contacts batchImportContacts | Typed validation, JSON/select, preview and error recovery |
| 8 | Apply a tag or list change to every contact matching a filter | Official v1.4.0 POST /contacts/bulk-actions | (generated endpoint) contacts createBulkContactAction | Typed validation, JSON/select, preview and error recovery |
| 9 | Check the progress of a bulk contact action | Official v1.4.0 GET /contacts/bulk-actions/{id} | (generated endpoint) contacts getBulkContactAction | Typed validation, JSON/select, preview and error recovery |
| 10 | List unsubscribed contacts | Official v1.4.0 GET /contacts/unsubscribed | (generated endpoint) contacts listUnsubscribedContacts | Typed validation, JSON/select, preview and error recovery |
| 11 | Unsubscribe a contact by email | Official v1.4.0 PATCH /unsubscribe | (generated endpoint) unsubscribe unsubscribeContact | Typed validation, JSON/select, preview and error recovery |
| 12 | List contact tags | Official v1.4.0 GET /contact-tags | (generated endpoint) contact-tags listContactTags | Typed validation, JSON/select, preview and error recovery |
| 13 | Create a contact tag | Official v1.4.0 POST /contact-tags | (generated endpoint) contact-tags createContactTag | Typed validation, JSON/select, preview and error recovery |
| 14 | Get a contact tag | Official v1.4.0 GET /contact-tags/{id} | (generated endpoint) contact-tags getContactTag | Typed validation, JSON/select, preview and error recovery |
| 15 | Update a contact tag | Official v1.4.0 PATCH /contact-tags/{id} | (generated endpoint) contact-tags updateContactTag | Typed validation, JSON/select, preview and error recovery |
| 16 | Delete a contact tag | Official v1.4.0 DELETE /contact-tags/{id} | (generated endpoint) contact-tags deleteContactTag | Typed validation, JSON/select, preview and error recovery |
| 17 | List a contact's tags | Official v1.4.0 GET /contacts/{contact_id}/tags | (generated endpoint) contacts listContactTagsForContact | Typed validation, JSON/select, preview and error recovery |
| 18 | Attach a tag to a contact | Official v1.4.0 POST /contacts/{contact_id}/tags/{tag_id} | (generated endpoint) contacts attachContactTag | Typed validation, JSON/select, preview and error recovery |
| 19 | Remove a tag from a contact | Official v1.4.0 DELETE /contacts/{contact_id}/tags/{tag_id} | (generated endpoint) contacts detachContactTag | Typed validation, JSON/select, preview and error recovery |
| 20 | List campaigns | Official v1.4.0 GET /campaigns | (generated endpoint) campaigns listCampaigns | Typed validation, JSON/select, preview and error recovery |
| 21 | Create a new campaign | Official v1.4.0 POST /campaigns | (generated endpoint) campaigns createCampaign | Typed validation, JSON/select, preview and error recovery |
| 22 | Get a specific campaign | Official v1.4.0 GET /campaigns/{id} | (generated endpoint) campaigns getCampaign | Typed validation, JSON/select, preview and error recovery |
| 23 | Update a draft campaign | Official v1.4.0 PATCH /campaigns/{id} | (generated endpoint) campaigns updateCampaign | Typed validation, JSON/select, preview and error recovery |
| 24 | Delete a draft campaign | Official v1.4.0 DELETE /campaigns/{id} | (generated endpoint) campaigns deleteCampaign | Typed validation, JSON/select, preview and error recovery |
| 25 | Send a campaign immediately | Official v1.4.0 POST /campaigns/{id}/send | (generated endpoint) campaigns sendCampaign | Typed validation, JSON/select, preview and error recovery |
| 26 | Get campaign performance statistics | Official v1.4.0 GET /campaigns/{id}/stats | (generated endpoint) campaigns getCampaignStats | Typed validation, JSON/select, preview and error recovery |
| 27 | List the contacts behind a campaign's engagement | Official v1.4.0 GET /campaigns/{id}/engagement | (generated endpoint) campaigns listCampaignEngagement | Typed validation, JSON/select, preview and error recovery |
| 28 | Clone a sent campaign at part of its original audience | Official v1.4.0 POST /campaigns/{id}/resend | (generated endpoint) campaigns resendCampaign | Typed validation, JSON/select, preview and error recovery |
| 29 | List forms | Official v1.4.0 GET /forms | (generated endpoint) forms listForms | Typed validation, JSON/select, preview and error recovery |
| 30 | Create a new form | Official v1.4.0 POST /forms | (generated endpoint) forms createForm | Typed validation, JSON/select, preview and error recovery |
| 31 | Get a specific form | Official v1.4.0 GET /forms/{id} | (generated endpoint) forms getForm | Typed validation, JSON/select, preview and error recovery |
| 32 | Update a form | Official v1.4.0 PATCH /forms/{id} | (generated endpoint) forms updateForm | Typed validation, JSON/select, preview and error recovery |
| 33 | Delete a form | Official v1.4.0 DELETE /forms/{id} | (generated endpoint) forms deleteForm | Typed validation, JSON/select, preview and error recovery |
| 34 | Get current user information | Official v1.4.0 GET /me | (generated endpoint) me getCurrentUser | Typed validation, JSON/select, preview and error recovery |
| 35 | List user contact fields | Official v1.4.0 GET /contact-fields | (generated endpoint) contact-fields listContactFields | Typed validation, JSON/select, preview and error recovery |
| 36 | Create a custom contact field | Official v1.4.0 POST /contact-fields | (generated endpoint) contact-fields createContactField | Typed validation, JSON/select, preview and error recovery |
| 37 | Get a specific contact field | Official v1.4.0 GET /contact-fields/{id} | (generated endpoint) contact-fields getContactField | Typed validation, JSON/select, preview and error recovery |
| 38 | Update a contact field label | Official v1.4.0 PATCH /contact-fields/{id} | (generated endpoint) contact-fields updateContactField | Typed validation, JSON/select, preview and error recovery |
| 39 | Delete a contact field | Official v1.4.0 DELETE /contact-fields/{id} | (generated endpoint) contact-fields deleteContactField | Typed validation, JSON/select, preview and error recovery |
| 40 | List contact lists | Official v1.4.0 GET /lists | (generated endpoint) lists listContactLists | Typed validation, JSON/select, preview and error recovery |
| 41 | Create a new contact list | Official v1.4.0 POST /lists | (generated endpoint) lists createContactList | Typed validation, JSON/select, preview and error recovery |
| 42 | Get a specific contact list | Official v1.4.0 GET /lists/{id} | (generated endpoint) lists getContactList | Typed validation, JSON/select, preview and error recovery |
| 43 | Update a contact list | Official v1.4.0 PATCH /lists/{id} | (generated endpoint) lists updateContactList | Typed validation, JSON/select, preview and error recovery |
| 44 | Delete a contact list | Official v1.4.0 DELETE /lists/{id} | (generated endpoint) lists deleteContactList | Typed validation, JSON/select, preview and error recovery |
| 45 | Get contacts in a list | Official v1.4.0 GET /lists/{list_id}/contacts | (generated endpoint) lists listContactsInList | Typed validation, JSON/select, preview and error recovery |
| 46 | Add a contact to a list | Official v1.4.0 POST /lists/{list_id}/contacts | (generated endpoint) lists addContactToList | Typed validation, JSON/select, preview and error recovery |
| 47 | Remove a contact from a list | Official v1.4.0 DELETE /lists/{list_id}/contacts/{contact_id} | (generated endpoint) lists removeContactFromList | Typed validation, JSON/select, preview and error recovery |
| 48 | List sender domains | Official v1.4.0 GET /domains | (generated endpoint) domains listDomains | Typed validation, JSON/select, preview and error recovery |
| 49 | Add a sender domain | Official v1.4.0 POST /domains | (generated endpoint) domains createDomain | Typed validation, JSON/select, preview and error recovery |
| 50 | Get domain with DNS records | Official v1.4.0 GET /domains/{id} | (generated endpoint) domains getDomain | Typed validation, JSON/select, preview and error recovery |
| 51 | Delete a sender domain | Official v1.4.0 DELETE /domains/{id} | (generated endpoint) domains deleteDomain | Typed validation, JSON/select, preview and error recovery |
| 52 | Validate domain DNS records | Official v1.4.0 POST /domains/{id}/validate | (generated endpoint) domains validateDomain | Typed validation, JSON/select, preview and error recovery |
| 53 | List automations | Official v1.4.0 GET /automations | (generated endpoint) automations listAutomations | Typed validation, JSON/select, preview and error recovery |
| 54 | Create an automation | Official v1.4.0 POST /automations | (generated endpoint) automations createAutomation | Typed validation, JSON/select, preview and error recovery |
| 55 | Get a specific automation | Official v1.4.0 GET /automations/{id} | (generated endpoint) automations getAutomation | Typed validation, JSON/select, preview and error recovery |
| 56 | Update an automation | Official v1.4.0 PATCH /automations/{id} | (generated endpoint) automations updateAutomation | Typed validation, JSON/select, preview and error recovery |
| 57 | Delete an automation | Official v1.4.0 DELETE /automations/{id} | (generated endpoint) automations deleteAutomation | Typed validation, JSON/select, preview and error recovery |
| 58 | Add an email to an automation | Official v1.4.0 POST /automations/{id}/emails | (generated endpoint) automations createAutomationEmail | Typed validation, JSON/select, preview and error recovery |
| 59 | Update an automation email | Official v1.4.0 PATCH /automation-emails/{id} | (generated endpoint) automation-emails updateAutomationEmail | Typed validation, JSON/select, preview and error recovery |
| 60 | Remove an email from an automation | Official v1.4.0 DELETE /automation-emails/{id} | (generated endpoint) automation-emails deleteAutomationEmail | Typed validation, JSON/select, preview and error recovery |
| 61 | Offline resource sync | Prior CLI; framework / official API | sendfox-pp-cli sync | Preserve worthwhile prior workflow with current contract and safety evidence |
| 62 | Local full-text search | Prior CLI; framework / official API | sendfox-pp-cli search | Preserve worthwhile prior workflow with current contract and safety evidence |
| 63 | Local SQL | Prior CLI; framework / official API | sendfox-pp-cli sql | Preserve worthwhile prior workflow with current contract and safety evidence |
| 64 | Local analytics | Prior CLI; framework / official API | sendfox-pp-cli analytics | Preserve worthwhile prior workflow with current contract and safety evidence |
| 65 | Runtime capability discovery | Prior CLI; framework / official API | sendfox-pp-cli capabilities | Preserve worthwhile prior workflow with current contract and safety evidence |
| 66 | CSV input audit | Prior CLI; framework / official API | sendfox-pp-cli contacts audit-csv | Preserve worthwhile prior workflow with current contract and safety evidence |
| 67 | CSV reconciliation | Prior CLI; framework / official API | sendfox-pp-cli contacts reconcile-csv | Preserve worthwhile prior workflow with current contract and safety evidence |
| 68 | Contact onboarding preview | Prior CLI; framework / official API | sendfox-pp-cli contacts onboard | Preserve worthwhile prior workflow with current contract and safety evidence |
| 69 | Guarded CSV batch import | Prior CLI; framework / official API | sendfox-pp-cli contacts import-csv | Preserve worthwhile prior workflow with current contract and safety evidence |
| 70 | Form handoff | Prior CLI; framework / official API | sendfox-pp-cli forms generate | Preserve worthwhile prior workflow with current contract and safety evidence |
| 71 | Webhook contract gap report | Prior CLI; framework / official API | sendfox-pp-cli webhooks handoff | Preserve worthwhile prior workflow with current contract and safety evidence |
| 72 | Account snapshot | Prior CLI; framework / official API | sendfox-pp-cli workflow account-snapshot | Preserve worthwhile prior workflow with current contract and safety evidence |
| 73 | Audience membership map | Prior CLI; framework / official API | sendfox-pp-cli workflow audience-map | Preserve worthwhile prior workflow with current contract and safety evidence |
| 74 | Audience hygiene | Prior CLI; framework / official API | sendfox-pp-cli workflow hygiene-report | Preserve worthwhile prior workflow with current contract and safety evidence |
| 75 | Campaign digest | Prior CLI; framework / official API | sendfox-pp-cli workflow campaign-digest | Preserve worthwhile prior workflow with current contract and safety evidence |
| 76 | Draft launch plan | Prior CLI; framework / official API | sendfox-pp-cli workflow launch-plan | Preserve worthwhile prior workflow with current contract and safety evidence |
