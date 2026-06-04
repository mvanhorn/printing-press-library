# GoDaddy Official Routes

Generated: 2026-05-22 10:26 UTC

Source specs: `godaddy/api-map/openapi/official/*.json`.

| API | Method | Path | Summary |
|---|---|---|---|
| abuse | `GET` | `/v1/abuse/tickets` | List all abuse tickets ids that match user provided filters |
| abuse | `POST` | `/v1/abuse/tickets` | Create a new abuse ticket |
| abuse | `GET` | `/v1/abuse/tickets/{ticketId}` | Return the abuse ticket data for a given ticket id |
| abuse | `GET` | `/v2/abuse/tickets` | List all abuse tickets ids that match user provided filters |
| abuse | `POST` | `/v2/abuse/tickets` | Create a new abuse ticket |
| abuse | `GET` | `/v2/abuse/tickets/{ticketId}` | Return the abuse ticket data for a given ticket id |
| aftermarket | `GET` | `/v1/customers/{customerId}/auctions/listings` | Get listings from GoDaddy Auctions |
| aftermarket | `DELETE` | `/v1/aftermarket/listings` | Remove listings from GoDaddy Auction |
| aftermarket | `POST` | `/v1/aftermarket/listings/expiry` | Add expiry listings into GoDaddy Auction |
| agreements | `GET` | `/v1/agreements` | Retrieve Legal Agreements for provided agreements keys |
| ans | `GET` | `/v1/agents` | Search the ANSName Registry with flexible criteria |
| ans | `POST` | `/v1/agents/register` | Register a new agent with the ANS |
| ans | `POST` | `/v1/agents/resolution` | Resolve an ANSName to an endpoint |
| ans | `GET` | `/v1/agents/{agentId}` | Get agent details |
| ans | `POST` | `/v1/agents/{agentId}/revoke` | Revoke an active agent or cancel a pending registration |
| ans | `POST` | `/v1/agents/{agentId}/verify-acme` | Trigger ACME validation |
| ans | `POST` | `/v1/agents/{agentId}/verify-dns` | Verify DNS records configured |
| ans | `GET` | `/v1/agents/{agentId}/certificates/identity` | Get agent's identity certificates |
| ans | `POST` | `/v1/agents/{agentId}/certificates/identity` | Submit identity certificate CSR |
| ans | `GET` | `/v1/agents/{agentId}/certificates/server` | Get agent's server certificates |
| ans | `POST` | `/v1/agents/{agentId}/certificates/server/renewal` | Submit server certificate renewal request |
| ans | `GET` | `/v1/agents/{agentId}/certificates/server/renewal` | Get pending renewal status |
| ans | `DELETE` | `/v1/agents/{agentId}/certificates/server/renewal` | Cancel pending renewal |
| ans | `POST` | `/v1/agents/{agentId}/certificates/server/renewal/verify-acme` | Verify ACME challenges for pending server cert renewal |
| ans | `GET` | `/v1/agents/{agentId}/csrs/{csrId}/status` | Get CSR status |
| ans | `GET` | `/v1/agents/events` | Retrieve ANS agent events |
| auctions | `POST` | `/v1/customers/{customerId}/aftermarket/listings/bids` | Places multiple bids with a single request. |
| certificates | `POST` | `/v1/certificates` | Create a pending order for certificate |
| certificates | `POST` | `/v1/certificates/validate` | Validate a pending order for certificate |
| certificates | `GET` | `/v1/certificates/{certificateId}` | Retrieve certificate details |
| certificates | `GET` | `/v1/certificates/{certificateId}/actions` | Retrieve all certificate actions |
| certificates | `POST` | `/v1/certificates/{certificateId}/email/{emailId}/resend` | Resend an email |
| certificates | `POST` | `/v1/certificates/{certificateId}/email/resend/{emailAddress}` | Add alternate email address |
| certificates | `POST` | `/v1/certificates/{certificateId}/email/{emailId}/resend/{emailAddress}` | Resend email to email address |
| certificates | `GET` | `/v1/certificates/{certificateId}/email/history` | Retrieve email history |
| certificates | `DELETE` | `/v1/certificates/{certificateId}/callback` | Unregister system callback |
| certificates | `GET` | `/v1/certificates/{certificateId}/callback` | Retrieve system stateful action callback url |
| certificates | `PUT` | `/v1/certificates/{certificateId}/callback` | Register of certificate action callback |
| certificates | `POST` | `/v1/certificates/{certificateId}/cancel` | Cancel a pending certificate |
| certificates | `GET` | `/v1/certificates/{certificateId}/download` | Download certificate |
| certificates | `POST` | `/v1/certificates/{certificateId}/reissue` | Reissue active certificate |
| certificates | `POST` | `/v1/certificates/{certificateId}/renew` | Renew active certificate |
| certificates | `POST` | `/v1/certificates/{certificateId}/revoke` | Revoke active certificate |
| certificates | `GET` | `/v1/certificates/{certificateId}/siteSeal` | Get Site seal |
| certificates | `POST` | `/v1/certificates/{certificateId}/verifyDomainControl` | Check Domain Control |
| certificates | `GET` | `/v2/certificates` | Search for certificate details by entitlement |
| certificates | `POST` | `/v2/certificates` | Create a pending order for certificate |
| certificates | `POST` | `/v2/certificates/{certificateId}/reissue` | Reissue active certificate |
| certificates | `GET` | `/v2/certificates/download` | Download certificate by entitlement |
| certificates | `GET` | `/v2/customers/{customerId}/certificates` | Retrieve customer's certificates |
| certificates | `GET` | `/v2/customers/{customerId}/certificates/{certificateId}` | Retrieve individual certificate details |
| certificates | `GET` | `/v2/customers/{customerId}/certificates/{certificateId}/domainVerifications` | Retrieve domain verification status |
| certificates | `GET` | `/v2/customers/{customerId}/certificates/{certificateId}/domainVerifications/{domain}` | Retrieve detailed information for supplied domain |
| certificates | `GET` | `/v2/customers/{customerId}/certificates/acme/externalAccountBinding` | Retrieves the external account binding for the specified customer |
| certificates | `GET` | `/v2/certificates/subscriptions/search` | Get a page of subscriptions by domain |
| certificates | `GET` | `/v2/certificates/subscription/{guid}` | GET a page of certificates for a specific domain product |
| countries | `GET` | `/v1/countries` | Retrieves summary country information for the provided marketId and filters |
| countries | `GET` | `/v1/countries/{countryKey}` | Retrieves country and summary state information for provided countryKey |
| domains | `GET` | `/v1/domains` | Retrieve a list of Domains for the specified Shopper |
| domains | `GET` | `/v1/domains/agreements` | Retrieve the legal agreement(s) required to purchase the specified TLD and add-ons |
| domains | `GET` | `/v1/domains/available` | Determine whether or not the specified domain is available for purchase |
| domains | `POST` | `/v1/domains/available` | Determine whether or not the specified domains are available for purchase |
| domains | `POST` | `/v1/domains/contacts/validate` | Validate the request body using the Domain Contact Validation Schema for specified domains. |
| domains | `POST` | `/v1/domains/purchase` | Purchase and register the specified Domain |
| domains | `GET` | `/v1/domains/purchase/schema/{tld}` | Retrieve the schema to be submitted when registering a Domain for the specified TLD |
| domains | `POST` | `/v1/domains/purchase/validate` | Validate the request body using the Domain Purchase Schema for the specified TLD |
| domains | `GET` | `/v1/domains/suggest` | Suggest alternate Domain names based on a seed Domain, a set of keywords, or the shopper's purchase history |
| domains | `GET` | `/v1/domains/tlds` | Retrieves a list of TLDs supported and enabled for sale |
| domains | `DELETE` | `/v1/domains/{domain}` | Cancel a purchased domain |
| domains | `GET` | `/v1/domains/{domain}` | Retrieve details for the specified Domain |
| domains | `PATCH` | `/v1/domains/{domain}` | Update details for the specified Domain |
| domains | `PATCH` | `/v1/domains/{domain}/contacts` | Update domain |
| domains | `DELETE` | `/v1/domains/{domain}/privacy` | Submit a privacy cancellation request for the given domain |
| domains | `POST` | `/v1/domains/{domain}/privacy/purchase` | Purchase privacy for a specified domain |
| domains | `PATCH` | `/v1/domains/{domain}/records` | Add the specified DNS Records to the specified Domain |
| domains | `PUT` | `/v1/domains/{domain}/records` | Replace all DNS Records for the specified Domain |
| domains | `GET` | `/v1/domains/{domain}/records/{type}/{name}` | Retrieve DNS Records for the specified Domain, optionally with the specified Type and/or Name |
| domains | `PUT` | `/v1/domains/{domain}/records/{type}/{name}` | Replace all DNS Records for the specified Domain with the specified Type and Name |
| domains | `DELETE` | `/v1/domains/{domain}/records/{type}/{name}` | Delete all DNS Records for the specified Domain with the specified Type and Name |
| domains | `PUT` | `/v1/domains/{domain}/records/{type}` | Replace all DNS Records for the specified Domain with the specified Type |
| domains | `POST` | `/v1/domains/{domain}/renew` | Renew the specified Domain |
| domains | `POST` | `/v1/domains/{domain}/transfer` | Purchase and start or restart transfer process |
| domains | `POST` | `/v1/domains/{domain}/verifyRegistrantEmail` | Re-send Contact E-mail Verification for specified Domain |
| domains | `GET` | `/v2/customers/{customerId}/domains/{domain}` | Retrieve details for the specified Domain |
| domains | `DELETE` | `/v2/customers/{customerId}/domains/{domain}/changeOfRegistrant` | Cancels a pending change of registrant request for a given domain |
| domains | `GET` | `/v2/customers/{customerId}/domains/{domain}/changeOfRegistrant` | Retrieve change of registrant information |
| domains | `PATCH` | `/v2/customers/{customerId}/domains/{domain}/dnssecRecords` | Add the specifed DNSSEC records to the domain |
| domains | `DELETE` | `/v2/customers/{customerId}/domains/{domain}/dnssecRecords` | Remove the specifed DNSSEC record from the domain |
| domains | `PUT` | `/v2/customers/{customerId}/domains/{domain}/nameServers` | Replaces the existing name servers on the domain. |
| domains | `GET` | `/v2/customers/{customerId}/domains/{domain}/privacy/forwarding` | Retrieve privacy email forwarding settings showing where emails are delivered |
| domains | `PATCH` | `/v2/customers/{customerId}/domains/{domain}/privacy/forwarding` | Update privacy email forwarding settings to determine how emails are delivered |
| domains | `POST` | `/v2/customers/{customerId}/domains/{domain}/redeem` | Purchase a restore for the given domain to bring it out of redemption |
| domains | `POST` | `/v2/customers/{customerId}/domains/{domain}/renew` | Renew the specified Domain |
| domains | `POST` | `/v2/customers/{customerId}/domains/{domain}/transfer` | Purchase and start or restart transfer process |
| domains | `GET` | `/v2/customers/{customerId}/domains/{domain}/transfer` | Query the current transfer status |
| domains | `POST` | `/v2/customers/{customerId}/domains/{domain}/transfer/validate` | Validate the request body using the Domain Transfer Schema for the specified TLD |
| domains | `POST` | `/v2/customers/{customerId}/domains/{domain}/transferInAccept` | Accepts the transfer in |
| domains | `POST` | `/v2/customers/{customerId}/domains/{domain}/transferInCancel` | Cancels the transfer in |
| domains | `POST` | `/v2/customers/{customerId}/domains/{domain}/transferInRestart` | Restarts transfer in request from the beginning |
| domains | `POST` | `/v2/customers/{customerId}/domains/{domain}/transferInRetry` | Retries the current transfer in request with supplied Authorization code |
| domains | `POST` | `/v2/customers/{customerId}/domains/{domain}/transferOut` | Initiate transfer out to another registrar for a .uk domain. |
| domains | `POST` | `/v2/customers/{customerId}/domains/{domain}/transferOutAccept` | Accept transfer out |
| domains | `POST` | `/v2/customers/{customerId}/domains/{domain}/transferOutReject` | Reject transfer out |
| domains | `DELETE` | `/v2/customers/{customerId}/domains/forwards/{fqdn}` | Submit a forwarding cancellation request for the given fqdn |
| domains | `GET` | `/v2/customers/{customerId}/domains/forwards/{fqdn}` | Retrieve the forwarding information for the given fqdn |
| domains | `PUT` | `/v2/customers/{customerId}/domains/forwards/{fqdn}` | Modify the forwarding information for the given fqdn |
| domains | `POST` | `/v2/customers/{customerId}/domains/forwards/{fqdn}` | Create a new forwarding configuration for the given FQDN |
| domains | `GET` | `/v2/customers/{customerId}/domains/{domain}/actions` | Retrieves a list of the most recent actions for the specified domain |
| domains | `DELETE` | `/v2/customers/{customerId}/domains/{domain}/actions/{type}` | Cancel the most recent user action for the specified domain |
| domains | `GET` | `/v2/customers/{customerId}/domains/{domain}/actions/{type}` | Retrieves the most recent action for the specified domain |
| domains | `GET` | `/v2/customers/{customerId}/domains/notifications` | Retrieve the next domain notification |
| domains | `GET` | `/v2/customers/{customerId}/domains/notifications/optIn` | Retrieve a list of notification types that are opted in |
| domains | `PUT` | `/v2/customers/{customerId}/domains/notifications/optIn` | Opt in to recieve notifications for the submitted notification types |
| domains | `GET` | `/v2/customers/{customerId}/domains/notifications/schemas/{type}` | Retrieve the schema for the notification data for the specified notification type |
| domains | `POST` | `/v2/customers/{customerId}/domains/notifications/{notificationId}/acknowledge` | Acknowledge a domain notification |
| domains | `POST` | `/v2/customers/{customerId}/domains/register` | Purchase and register the specified Domain |
| domains | `GET` | `/v2/customers/{customerId}/domains/register/schema/{tld}` | Retrieve the schema to be submitted when registering a Domain for the specified TLD |
| domains | `POST` | `/v2/customers/{customerId}/domains/register/validate` | Validate the request body using the Domain Registration Schema for the specified TLD |
| domains | `GET` | `/v2/domains/maintenances` | Retrieve a list of upcoming system Maintenances |
| domains | `GET` | `/v2/domains/maintenances/{maintenanceId}` | Retrieve the details for an upcoming system Maintenances |
| domains | `GET` | `/v2/domains/usage/{yyyymm}` | Retrieve api usage request counts for a specific year/month.  The data is retained for a period of three months. |
| domains | `PATCH` | `/v2/customers/{customerId}/domains/{domain}/contacts` | Update domain contacts |
| domains | `POST` | `/v2/customers/{customerId}/domains/{domain}/regenerateAuthCode` | Regenerate the auth code for the given domain |
| orders | `GET` | `/v1/orders` | Retrieve a list of orders for the authenticated shopper. Only one filter may be used at a time |
| orders | `GET` | `/v1/orders/{orderId}` | Retrieve details for specified order |
| parking | `GET` | `/v1/customers/{customerId}/parking/metrics` | Returns a list of parking metrics for the specified customer, using specified filters |
| parking | `GET` | `/v1/customers/{customerId}/parking/metricsByDomain` | Returns a list of domain metrics for the specified customer and portfolio, using specified filters |
| shoppers | `POST` | `/v1/shoppers/subaccount` | Create a Subaccount owned by the authenticated Reseller |
| shoppers | `GET` | `/v1/shoppers/{shopperId}` | Get details for the specified Shopper |
| shoppers | `POST` | `/v1/shoppers/{shopperId}` | Update details for the specified Shopper |
| shoppers | `DELETE` | `/v1/shoppers/{shopperId}` | Request the deletion of a shopper profile |
| shoppers | `GET` | `/v1/shoppers/{shopperId}/status` | Get details for the specified Shopper |
| shoppers | `PUT` | `/v1/shoppers/{shopperId}/factors/password` | Set subaccount's password |
| subscriptions | `GET` | `/v1/subscriptions` | Retrieve a list of Subscriptions for the specified Shopper |
| subscriptions | `GET` | `/v1/subscriptions/productGroups` | Retrieve a list of ProductGroups for the specified Shopper |
| subscriptions | `DELETE` | `/v1/subscriptions/{subscriptionId}` | Cancel the specified Subscription |
| subscriptions | `GET` | `/v1/subscriptions/{subscriptionId}` | Retrieve details for the specified Subscription |
| subscriptions | `PATCH` | `/v1/subscriptions/{subscriptionId}` | Update details for the specified Subscription |
