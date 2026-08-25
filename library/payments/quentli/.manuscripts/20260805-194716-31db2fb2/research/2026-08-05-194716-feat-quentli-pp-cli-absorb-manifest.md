## Absorb Manifest

### Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | List customers | GET /v1/customers | (generated endpoint) customers list | Rich qs `filter`, offline search over mirror |
| 2 | Create customer | POST /v1/customers | (generated endpoint) customers create | scriptable, dry-run |
| 3 | Get customer | GET /v1/customers/{id} | (generated endpoint) customers get | --json --select |
| 4 | Update customer | PATCH /v1/customers/{id} | (generated endpoint) customers update | dry-run |
| 5 | Get customer payment methods | GET /v1/customers/{customerId}/payment_methods | (generated endpoint) customers payment-methods | confirmed only |
| 6 | Create customer tax profile | POST /v1/customers/{customerId}/tax-profiles | (generated endpoint) customers tax-profiles create | SAT fiscal data |
| 7 | Update customer tax profile | PATCH /v1/customers/{customerId}/tax-profiles/{id} | (generated endpoint) customers tax-profiles update |  |
| 8 | Delete customer tax profile | DELETE /v1/customers/{customerId}/tax-profiles/{id} | (generated endpoint) customers tax-profiles delete | dry-run |
| 9 | List payment concepts | GET /v1/payment-concepts | (generated endpoint) payment-concepts list | catalog |
| 10 | Create payment concept | POST /v1/payment-concepts | (generated endpoint) payment-concepts create |  |
| 11 | Get payment concept | GET /v1/payment-concepts/{id} | (generated endpoint) payment-concepts get |  |
| 12 | Update payment concept | PATCH /v1/payment-concepts/{id} | (generated endpoint) payment-concepts update |  |
| 13 | List discounts | GET /v1/discounts | (generated endpoint) discounts list |  |
| 14 | Create discount | POST /v1/discounts | (generated endpoint) discounts create |  |
| 15 | Get discount | GET /v1/discounts/{id} | (generated endpoint) discounts get |  |
| 16 | Update discount | PATCH /v1/discounts/{id} | (generated endpoint) discounts update |  |
| 17 | Delete discount | DELETE /v1/discounts/{id} | (generated endpoint) discounts delete | dry-run |
| 18 | Create invoice | POST /v1/invoices | (generated endpoint) invoices create | flagship |
| 19 | List invoices | GET /v1/invoices | (generated endpoint) invoices list |  |
| 20 | Get invoice | GET /v1/invoices/{id} | (generated endpoint) invoices get |  |
| 21 | Update invoice | PATCH /v1/invoices/{id} | (generated endpoint) invoices update |  |
| 22 | Cancel invoice | DELETE /v1/invoices/{id} | (generated endpoint) invoices cancel | dry-run |
| 23 | Get payment link | GET /v1/invoices/{id}/payment-link | (generated endpoint) invoices payment-link |  |
| 24 | Retry invoice payment | POST /v1/invoices/{id}/try-payment | (generated endpoint) invoices try-payment |  |
| 25 | Mark invoice as paid | POST /v1/invoices/{id}/mark-paid | (generated endpoint) invoices mark-paid | dry-run |
| 26 | Apply invoice payment | POST /v1/invoices/{id}/payments/apply | (generated endpoint) invoices payments apply |  |
| 27 | Send invoice reminder | POST /v1/invoices/{id}/reminders | (generated endpoint) invoices reminders | dry-run |
| 28 | Create payment session | POST /v1/payment-sessions | (generated endpoint) payment-sessions create | FLAGSHIP hosted pay link |
| 29 | Get payment session | GET /v1/payment-sessions/{id} | (generated endpoint) payment-sessions get |  |
| 30 | Create setup session | POST /v1/setup-sessions | (generated endpoint) setup-sessions create | enroll payment method |
| 31 | Create payment | POST /v1/payments | (generated endpoint) payments create | direct charge |
| 32 | List payments | GET /v1/payments | (generated endpoint) payments list |  |
| 33 | Get payment | GET /v1/payments/{id} | (generated endpoint) payments get |  |
| 34 | Send payment receipt | POST /v1/payments/{id}/receipt | (generated endpoint) payments receipt | dry-run |
| 35 | Create refund | POST /v1/refunds | (generated endpoint) refunds create | dry-run |
| 36 | List subscriptions | GET /v1/subscriptions | (generated endpoint) subscriptions list |  |
| 37 | Create subscription | POST /v1/subscriptions | (generated endpoint) subscriptions create | recurring |
| 38 | Get subscription | GET /v1/subscriptions/{id} | (generated endpoint) subscriptions get |  |
| 39 | Update subscription | PATCH /v1/subscriptions/{id} | (generated endpoint) subscriptions update |  |
| 40 | Cancel subscription | DELETE /v1/subscriptions/{id} | (generated endpoint) subscriptions cancel | dry-run |
| 41 | Delete payment method | DELETE /v1/payment-methods/{id} | (generated endpoint) payment-methods delete | dry-run |
| 42 | Create tax invoice | POST /v1/tax-invoices | (generated endpoint) tax-invoices create | SAT CFDI |
| 43 | List tax invoices | GET /v1/tax-invoices | (generated endpoint) tax-invoices list |  |
| 44 | Get tax invoice | GET /v1/tax-invoices/{id} | (generated endpoint) tax-invoices get |  |
| 45 | Cancel tax invoice | POST /v1/tax-invoices/{id}/cancel | (generated endpoint) tax-invoices cancel | dry-run |
| 46 | List webhooks | GET /v1/webhooks | (generated endpoint) webhooks list |  |
| 47 | Create webhook | POST /v1/webhooks | (generated endpoint) webhooks create |  |
| 48 | Get webhook | GET /v1/webhooks/{id} | (generated endpoint) webhooks get |  |
| 49 | Update webhook | PATCH /v1/webhooks/{id} | (generated endpoint) webhooks update |  |
| 50 | Delete webhook | DELETE /v1/webhooks/{id} | (generated endpoint) webhooks delete | dry-run |
| 51 | List webhook events | GET /v1/webhook-events | (generated endpoint) webhook-events list |  |
| 52 | Retry webhook event | POST /v1/webhook-events/{id}/retry | (generated endpoint) webhook-events retry | dry-run |
| 53 | Create auth link | POST /v1/auth-links | (generated endpoint) auth-links create | portal sso link |
| 54 | Create customer portal session | POST /v1/customer-portal-session | (generated endpoint) customer-portal-session create |  |

### Transcendence (only possible with our approach)
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|------------------------|------------------|
| 1 | Collection / dunning queue | dunning | hand-code | Requires local join across invoices + customers + payments to compute outstanding/overdue and a next action offline | For the whole-portfolio collection queue. Use 'customer balance <id>' for a single customer's drill-down. |
| 2 | CFDI tax-invoice reconciliation | reconcile | hand-code | Requires local cross-check of completed/refunded payments and paid invoices against SAT tax-invoice status | none |
| 3 | Subscriptions at-risk | subs at-risk | hand-code | Requires local join of subscriptions + payment methods + failed payments to flag involuntary churn risk | Whole-portfolio recovery queue; use 'customer balance <id>' for a single subscription's details. |
| 4 | Revenue report | revenue | hand-code | Aggregates local payments + refunds by status/type/currency for net collected vs returned | none |
| 5 | Webhook delivery health | webhooks health | hand-code | Aggregates local webhook-events by status and surfaces PAYMENT_ATTEMPT_FAILED as an operational alert | none |
| 6 | Customer balance drill-down | customer balance | hand-code | Single-key multi-entity local join renders a one-screen financial snapshot for one customer | For one customer's financial snapshot. Use 'dunning' for the whole-portfolio collection queue. |
