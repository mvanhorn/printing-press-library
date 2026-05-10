# Pennylane CLI

CLI for Pennylane's accounting API with AR aging, cash runway, VAT preview, FEC validation, and bulk invoice operations.

## Install

The recommended path installs both the `accounting-pp-cli` binary and the `pp-accounting` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install accounting
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install accounting --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/accounting-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-accounting --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-accounting --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-accounting skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-accounting. The skill defines how its required CLI can be installed.
```

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Set Up Credentials

Get your access token from your API provider's developer portal, then store it:

```bash
accounting-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set it via environment variable:

```bash
export ACCOUNTING_OAUTH2="your-token-here"
```

### 3. Verify Setup

```bash
accounting-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
accounting-pp-cli external company-fiscal-years
```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Intelligence financiere locale
- **`ar aging`** — Classe toutes les creances clients par tranche de retard (0-30j, 30-60j, 60-90j, 90j+) en une commande.

  _Rapport mensuel obligatoire pour tout cabinet comptable — genere en <1s offline._

  ```bash
  accounting-pp-cli ar aging --buckets 0,30,60,90 --json --agent
  ```
- **`cash dso`** — Calcule le delai moyen de paiement par client et globalement sur une periode glissante.

  _Identifie les mauvais payeurs avant qu'ils deviennent un probleme de tresorerie._

  ```bash
  accounting-pp-cli cash dso --rolling 90 --json --agent
  ```
- **`cash runway`** — Projette la position de tresorerie jour par jour sur N jours en combinant AR ouvert et AP ouvert.

  _Repond a la question cle : 'quand est-ce que je manque de cash ?' sans Excel._

  ```bash
  accounting-pp-cli cash runway --horizon 90 --json --agent
  ```
- **`audit anomalies`** — Detecte les transactions en double, montants ronds sans facture correspondante et autres irregularites.

  _Une fraude ou erreur detectee justifie immediatement l'outil._

  ```bash
  accounting-pp-cli audit anomalies --sigma 2 --json --agent
  ```
- **`clients rank`** — Classe les clients par CA, nombre de factures ou marge sur une periode.

  _Identifie les 3 clients qui font 80% du CA — information strategique en 1 commande._

  ```bash
  accounting-pp-cli clients rank --by revenue --ytd --json --agent
  ```
- **`ap schedule`** — Classe les factures fournisseurs ouvertes par priorite de paiement selon echeance et criticite.

  _Les PME contraintes en cash ont besoin de prioriser leurs paiements sans Excel._

  ```bash
  accounting-pp-cli ap schedule --horizon 60 --json --agent
  ```

### Conformite fiscale française
- **`vat preview`** — Agrege la TVA collectee (ventes) et deductible (achats) par taux pour une periode, pret a copier dans impots.gouv.fr.

  _Elimine 2h de travail manuel a chaque declaration trimestrielle TVA._

  ```bash
  accounting-pp-cli vat preview --period 2026-Q1 --json --agent
  ```
- **`fec validate`** — Valide un fichier FEC DGFiP : equilibre debit/credit, champs obligatoires, monotonie des dates.

  _Un FEC invalide soumis au DGFiP = controle fiscal. Validation en <1s previent le probleme._

  ```bash
  accounting-pp-cli fec validate --file FEC2025.txt --json
  ```
- **`yearend check`** — Execute une checklist de 4 controles de cloture : transactions non rapprochees, factures draft, avoirs ouverts.

  _La cloture annuelle est le moment le plus risque — checklist automatisee en 1 commande._

  ```bash
  accounting-pp-cli yearend check --fiscal-year 2025 --json --agent
  ```

### Operations en masse
- **`invoice bulk-create`** — Cree N factures depuis un CSV avec validation dry-run, correspondance client fuzzy et throttling API.

  _Remplace la creation manuelle de 50-500 factures en fin de mois — raison principale d'abandon de Pennylane._

  ```bash
  accounting-pp-cli invoice bulk-create --file factures.csv --dry-run
  ```
- **`invoice check-recurring`** — Detecte les mois manquants et les ecarts de montant sur les factures a periodicite reguliere.

  _Les ecarts de facturation recurrente sont une fuite de CA silencieuse._

  ```bash
  accounting-pp-cli invoice check-recurring --tolerance 5 --months 6 --json
  ```
- **`ar remind`** — Liste les creances a relancer avec deduplication automatique des relances recentes.

  _La relance manuelle est la #1 perte de temps des freelances français._

  ```bash
  accounting-pp-cli ar remind --overdue-days 30 --dry-run --json
  ```

## Usage

Run `accounting-pp-cli --help` for the full command reference and flag list.

## Commands

### external

Manage external

- **`accounting-pp-cli external company-fiscal-years`** - This endpoint returns a list of fiscal years of the company.

**DEPRECATED BEHAVIOR:**
By default, returns fiscal years ordered by ascending `start` date.

**NEW BEHAVIOR:**
By default, returns fiscal years ordered by descending IDs
A new `sort` query parameter is now available allowing to sort by `id` or `start` attributes.
For more details, see our API documentation https://pennylane.readme.io/docs/2026-api-changes-guide for migration instructions.


> ℹ️
> This endpoint requires the following scope: `fiscal_years:readonly`
- **`accounting-pp-cli external create-customer-invoice-einvoice-import`** - Import a customer invoice from an e-invoice file (Factur-X format).
The file must be a valid Factur-X PDF.
Optionally provide `invoice_options` to pre-fill customer and line-level data.
Invoice line `e_invoice_line_id` must match Factur-X BT-126 (LineID).


> ℹ️
> This endpoint requires the following scope: `customer_invoices:all`
- **`accounting-pp-cli external create-customer-invoice-from-quote`** - This endpoint allows you to create a customer invoice from an existing quote.
The invoice will inherit the quote's data (customer, lines, etc.).


> ℹ️
> This endpoint requires the following scope: `customer_invoices:all`
- **`accounting-pp-cli external create-einvoice-import`** - This endpoint allows you to import an e-invoice.

> ‼️
> This endpoint is **DEPRECATED**
> As an alternative, please use either:
>   - [Import a customer e-invoice](https://pennylane.readme.io/reference/createcustomerinvoiceeinvoiceimport) endpoint
>   - [Import a supplier e-invoice](https://pennylane.readme.io/reference/createsupplierinvoiceeinvoiceimport) endpoint

> ℹ️
> This endpoint requires the following scope: `e_invoices:all`
- **`accounting-pp-cli external create-purchase-request-import`** - Import a purchase order. This will create a purchase request with an existing purchase order attached.
The purchase request will be **automatically validated**.


> ℹ️
> This endpoint requires the following scope: `purchase_requests:all`
- **`accounting-pp-cli external create-supplier-invoice-einvoice-import`** - Import a supplier invoice from an e-invoice file (Factur-X format).
The file must be a valid Factur-X PDF.
Optionally provide `invoice_options` to pre-fill supplier and line-level data.
Invoice line `e_invoice_line_id` must match Factur-X BT-126 (LineID).


> ℹ️
> This endpoint requires the following scope: `supplier_invoices:all`
- **`accounting-pp-cli external create-transaction`** - Create a banking transaction

> ℹ️
> This endpoint requires the following scope: `transactions:all`
- **`accounting-pp-cli external delete-customer-invoice-matched-transactions`** - This endpoint allows you to unmatch a transaction to a customer invoice. It is not applicable for draft invoices.


> ℹ️
> This endpoint requires the following scope: `customer_invoices:all`
- **`accounting-pp-cli external delete-customer-invoices`** - Delete a draft customer invoice or draft credit note

> ℹ️
> This endpoint requires the following scope: `customer_invoices:all`
- **`accounting-pp-cli external delete-ledger-entry-lines-unletter`** - This endpoint lets you unletter ledger entry lines.

> ℹ️
> This endpoint requires one of the following scopes: `ledger (DEPRECATED)`, `ledger_entries:all`
- **`accounting-pp-cli external delete-sepa-mandate`** - This endpoint allows you to delete a specific SEPA mandate

> ℹ️
> This endpoint requires the following scope: `customer_mandates:all`
- **`accounting-pp-cli external delete-supplier-invoice-matched-transactions`** - This endpoint allows you to unmatch a transaction to a supplier
invoice.


> ℹ️
> This endpoint requires the following scope: `supplier_invoices:all`
- **`accounting-pp-cli external delete-webhook-subscription`** - This endpoint allows you to delete the webhook subscription for the authenticated
token. Each token (developer token or OAuth application) can only have one webhook subscription.
- **`accounting-pp-cli external export-analytical-general-ledger`** - This endpoint allows you to create an Analytical General Ledger export. The generated export file is an xlsx file, using the in-line analytical mode by default.

> ℹ️
> This endpoint requires the following scope: `exports:agl`
- **`accounting-pp-cli external export-fec`** - This endpoint allows you to create a FEC export

> ℹ️
> This endpoint requires the following scope: `exports:fec`
- **`accounting-pp-cli external export-general-ledger`** - This endpoint allows you to create a General Ledger export. The generated export file is an xlsx file.

> ℹ️
> This endpoint requires the following scope: `exports:gl`
- **`accounting-pp-cli external finalize-customer-invoice`** - Convert the draft customer invoice or credit note into a finalized
one. Once finalized, the resource can no longer be edited.


> ℹ️
> This endpoint requires the following scope: `customer_invoices:all`
- **`accounting-pp-cli external get-analytical-general-ledger-export`** - The endpoint returns a specific Analytical General Ledger export. The export file is an xlsx file, using the in-line analytical mode.

> ℹ️
> This endpoint requires the following scope: `exports:agl`
- **`accounting-pp-cli external get-bank-account`** - Retrieve a bank account

> ℹ️
> This endpoint requires one of the following scopes: `bank_accounts:all`, `bank_accounts:readonly`
- **`accounting-pp-cli external get-bank-accounts`** - List bank_accounts

> ℹ️
> This endpoint requires one of the following scopes: `bank_accounts:all`, `bank_accounts:readonly`
- **`accounting-pp-cli external get-bank-establishments`** - List bank establishments

> ℹ️
> This endpoint requires the following scope: `bank_establishments:readonly`
- **`accounting-pp-cli external get-billing-subscription`** - This endpoint returns a specific billing subscription.

> ℹ️
> This endpoint requires one of the following scopes: `billing_subscriptions:all`, `billing_subscriptions:readonly`
- **`accounting-pp-cli external get-billing-subscription-invoice-line-sections`** - List the invoice line sections of a billing subscription

> ℹ️
> This endpoint requires one of the following scopes: `billing_subscriptions:all`, `billing_subscriptions:readonly`
- **`accounting-pp-cli external get-billing-subscription-invoice-lines`** - List invoice lines for a billing subscription

> ℹ️
> This endpoint requires one of the following scopes: `billing_subscriptions:all`, `billing_subscriptions:readonly`
- **`accounting-pp-cli external get-billing-subscriptions`** - This endpoint returns a list of subscriptions.

> ℹ️
> This endpoint requires one of the following scopes: `billing_subscriptions:all`, `billing_subscriptions:readonly`
- **`accounting-pp-cli external get-categories`** - List categories

> ℹ️
> This endpoint requires one of the following scopes: `categories:all`, `categories:readonly`
- **`accounting-pp-cli external get-category`** - This endpoint returns a specific category.

> ℹ️
> This endpoint requires one of the following scopes: `categories:all`, `categories:readonly`
- **`accounting-pp-cli external get-category-group`** - This endpoint returns a specific category group.

> ℹ️
> This endpoint requires one of the following scopes: `categories:all`, `categories:readonly`
- **`accounting-pp-cli external get-category-group-categories`** - List categories of a category group

> ℹ️
> This endpoint requires one of the following scopes: `categories:all`, `categories:readonly`
- **`accounting-pp-cli external get-category-groups`** - This endpoint returns a list of category groups

> ℹ️
> This endpoint requires one of the following scopes: `categories:all`, `categories:readonly`
- **`accounting-pp-cli external get-commercial-document`** - This endpoint retrieves a commercial document.

> ℹ️
> This endpoint requires one of the following scopes: `commercial_documents:all`, `commercial_documents:readonly`
- **`accounting-pp-cli external get-commercial-document-appendices`** - List appendices of a commercial document

> ℹ️
> This endpoint requires one of the following scopes: `commercial_documents:all`, `commercial_documents:readonly`
- **`accounting-pp-cli external get-commercial-document-invoice-line-sections`** - List invoice line sections for a commercial document

> ℹ️
> This endpoint requires one of the following scopes: `commercial_documents:all`, `commercial_documents:readonly`
- **`accounting-pp-cli external get-commercial-document-invoice-lines`** - List invoice lines for a commercial document

> ℹ️
> This endpoint requires one of the following scopes: `commercial_documents:all`, `commercial_documents:readonly`
- **`accounting-pp-cli external get-company-customer`** - This endpoint returns a company customer.

> ℹ️
> This endpoint requires one of the following scopes: `customers:all`, `customers:readonly`
- **`accounting-pp-cli external get-customer`** - This endpoint returns a customer.

> ℹ️
> This endpoint requires one of the following scopes: `customers:all`, `customers:readonly`
- **`accounting-pp-cli external get-customer-categories`** - List categories of a customer

> ℹ️
> This endpoint requires one of the following scopes: `customers:readonly`, `customers:all`
- **`accounting-pp-cli external get-customer-changes`** - Returns the list of changes based on the provided `start_date`.
If no `start_date` is provided it returns the oldest set of recorded changes.
Changes for the last 4 weeks are retained. The items will be returned using
`processed_at` in ASC order (oldest first).


> ℹ️
> This endpoint requires one of the following scopes: `customers:all`, `customers:readonly`
- **`accounting-pp-cli external get-customer-contacts`** - List contacts of a customer

> ℹ️
> This endpoint requires one of the following scopes: `customers:all`, `customers:readonly`
- **`accounting-pp-cli external get-customer-invoice`** - Retrieve a customer invoice or a credit note

> ℹ️
> This endpoint requires one of the following scopes: `customer_invoices:all`, `customer_invoices:readonly`
- **`accounting-pp-cli external get-customer-invoice-appendices`** - List appendices of a customer invoice

> ℹ️
> This endpoint requires one of the following scopes: `customer_invoices:all`, `customer_invoices:readonly`
- **`accounting-pp-cli external get-customer-invoice-categories`** - List categories of a customer invoice

> ℹ️
> This endpoint requires one of the following scopes: `customer_invoices:all`, `customer_invoices:readonly`
- **`accounting-pp-cli external get-customer-invoice-custom-header-fields`** - List custom header fields for a customer invoice

> ℹ️
> This endpoint requires one of the following scopes: `customer_invoices:all`, `customer_invoices:readonly`
- **`accounting-pp-cli external get-customer-invoice-invoice-line-sections`** - List invoice line sections for a customer invoice

> ℹ️
> This endpoint requires one of the following scopes: `customer_invoices:all`, `customer_invoices:readonly`
- **`accounting-pp-cli external get-customer-invoice-invoice-lines`** - List invoice lines for a customer invoice

> ℹ️
> This endpoint requires one of the following scopes: `customer_invoices:all`, `customer_invoices:readonly`
- **`accounting-pp-cli external get-customer-invoice-matched-transactions`** - List matched transactions for a customer invoice

> ℹ️
> This endpoint requires one of the following scopes: `customer_invoices:all`, `customer_invoices:readonly`
- **`accounting-pp-cli external get-customer-invoice-payments`** - List payments for a customer invoice

> ℹ️
> This endpoint requires one of the following scopes: `customer_invoices:all`, `customer_invoices:readonly`
- **`accounting-pp-cli external get-customer-invoice-templates`** - List customer invoice templates

> ℹ️
> This endpoint requires the following scope: `customer_invoice_templates:readonly`
- **`accounting-pp-cli external get-customer-invoices`** - List customer invoices and credit notes

> ℹ️
> This endpoint requires one of the following scopes: `customer_invoices:all`, `customer_invoices:readonly`
- **`accounting-pp-cli external get-customer-invoices-changes`** - Returns the list of changes based on the provided `start_date`.
If no `start_date` is provided it returns the oldest set of recorded changes.
Changes for the last 4 weeks are retained. The items will be returned using
`processed_at` in ASC order (oldest first).


> ℹ️
> This endpoint requires one of the following scopes: `customer_invoices:all`, `customer_invoices:readonly`
- **`accounting-pp-cli external get-customers`** - This endpoint returns a list of both company and individual customers

> ℹ️
> This endpoint requires one of the following scopes: `customers:all`, `customers:readonly`
- **`accounting-pp-cli external get-fec-export`** - The endpoint returns a specific FEC export

> ℹ️
> This endpoint requires the following scope: `exports:fec`
- **`accounting-pp-cli external get-file-attachments`** - List attachments

> ‼️
> This endpoint is **DEPRECATED**

> ℹ️
> This endpoint requires one of the following scopes: `file_attachments:all`, `file_attachments:readonly`
- **`accounting-pp-cli external get-general-ledger-export`** - The endpoint returns a specific General Ledger export. The export file is an xlsx file.

> ℹ️
> This endpoint requires the following scope: `exports:gl`
- **`accounting-pp-cli external get-gocardless-mandate`** - This endpoint allows you to retrieve a specific Gocardless mandate by ID.

> ℹ️
> This endpoint requires one of the following scopes: `customer_mandates:all`, `customer_mandates:readonly`
- **`accounting-pp-cli external get-gocardless-mandates`** - List gocardless mandates

> ℹ️
> This endpoint requires one of the following scopes: `customer_mandates:all`, `customer_mandates:readonly`
- **`accounting-pp-cli external get-individual-customer`** - This endpoint returns an individual customer.

> ℹ️
> This endpoint requires one of the following scopes: `customers:all`, `customers:readonly`
- **`accounting-pp-cli external get-journal`** - Retrieve a journal

**NEW BEHAVIOR :**
The old `ledger` scope will only work on the old behavior system. As soon as you opt in to the new version, or when the sunset phase starts and you haven't explicitly opted out of the old behavior, the ledger scope will no longer work.
For more details, see our API documentation https://pennylane.readme.io/docs/2026-api-changes-guide for migration instructions.


> ℹ️
> This endpoint requires one of the following scopes: `ledger (DEPRECATED)`, `journals:readonly`, `journals:all`
- **`accounting-pp-cli external get-journals`** - List journals

**DEPRECATED BEHAVIOR:**
By default, returns journals ordered by ascending IDs

**NEW BEHAVIOR:**
By default, returns journals ordered by descending IDs
The old `ledger` scope will only work on the old behavior system. As soon as you opt in to the new version, or when the sunset phase starts and you haven't explicitly opted out of the old behavior, the ledger scope will no longer work.
For more details, see our API documentation https://pennylane.readme.io/docs/2026-api-changes-guide for migration instructions.


> ℹ️
> This endpoint requires one of the following scopes: `ledger (DEPRECATED)`, `journals:readonly`, `journals:all`
- **`accounting-pp-cli external get-ledger-account`** - Get a ledger account

**NEW BEHAVIOR :**
The old `ledger` scope will only work on the old behavior system. As soon as you opt in to the new version, or when the sunset phase starts and you haven't explicitly opted out of the old behavior, the ledger scope will no longer work.
For more details, see our API documentation https://pennylane.readme.io/docs/2026-api-changes-guide for migration instructions.


> ℹ️
> This endpoint requires one of the following scopes: `ledger (DEPRECATED)`, `ledger_accounts:readonly`, `ledger_accounts:all`
- **`accounting-pp-cli external get-ledger-accounts`** - List Ledger Accounts

**DEPRECATED BEHAVIOR:**
By default, returns ledger accounts ordered by ascending IDs

**NEW BEHAVIOR:**
By default, returns ledger accounts ordered by descending IDs
The old `ledger` scope will only work on the old behavior system. As soon as you opt in to the new version, or when the sunset phase starts and you haven't explicitly opted out of the old behavior, the ledger scope will no longer work.
For more details, see our API documentation https://pennylane.readme.io/docs/2026-api-changes-guide for migration instructions.


> ℹ️
> This endpoint requires one of the following scopes: `ledger (DEPRECATED)`, `ledger_accounts:readonly`, `ledger_accounts:all`
- **`accounting-pp-cli external get-ledger-attachments`** - List attachments

> ‼️
> This endpoint is **DEPRECATED**
> As soon as you opt in to the new version, or when the sunset phase starts and you haven't explicitly opted out of the old behavior, this endpoint will no longer work.
For more details, see our API documentation https://pennylane.readme.io/docs/2026-api-changes-guide for migration instructions.

> ℹ️
> This endpoint requires the following scope: `ledger`
- **`accounting-pp-cli external get-ledger-entries`** - Returns a list of ledger entries.
**DEPRECATED BEHAVIOR:**
Draft entries are filtered out.
By default, entries from fiscal periods that are closed or frozen are excluded.
However, if a 'date' filter is provided, it will return all entries within the specified date range, even if they fall within a closed or frozen fiscal period.


**NEW BEHAVIOR :**
By default, all entries (including draft ones) are rendered regardless of their fiscal year status (open, closed, or frozen).
The old `ledger` scope will only work on the old behavior system. As soon as you opt in to the new version, or when the sunset phase starts and you haven't explicitly opted out of the old behavior, the ledger scope will no longer work.
For more details, see our API documentation https://pennylane.readme.io/docs/2026-api-changes-guide for migration instructions.

> ℹ️
> This endpoint requires one of the following scopes: `ledger (DEPRECATED)`, `ledger_entries:readonly`, `ledger_entries:all`
- **`accounting-pp-cli external get-ledger-entries-ledger-entry-lines`** - List ledger entry lines of a Ledger Entry

**DEPRECATED BEHAVIOR:**
By default, returns ledger entry lines ordered by ascending IDs

**NEW BEHAVIOR:**
By default, returns ledger entry lines ordered by descending IDs
The old `ledger` scope will only work on the old behavior system. As soon as you opt in to the new version, or when the sunset phase starts and you haven't explicitly opted out of the old behavior, the ledger scope will no longer work.
For more details, see our API documentation https://pennylane.readme.io/docs/2026-api-changes-guide for migration instructions.


> ℹ️
> This endpoint requires one of the following scopes: `ledger (DEPRECATED)`, `ledger_entries:readonly`, `ledger_entries:all`
- **`accounting-pp-cli external get-ledger-entry`** - Retrieve a ledger entry

**NEW BEHAVIOR :**
The old `ledger` scope will only work on the old behavior system. As soon as you opt in to the new version, or when the sunset phase starts and you haven't explicitly opted out of the old behavior, the ledger scope will no longer work.
For more details, see our API documentation https://pennylane.readme.io/docs/2026-api-changes-guide for migration instructions.


> ℹ️
> This endpoint requires one of the following scopes: `ledger (DEPRECATED)`, `ledger_entries:readonly`, `ledger_entries:all`
- **`accounting-pp-cli external get-ledger-entry-line`** - Retrieve a ledger entry line

> ℹ️
> This endpoint requires one of the following scopes: `ledger (DEPRECATED)`, `ledger_entries:readonly`, `ledger_entries:all`
- **`accounting-pp-cli external get-ledger-entry-line-changes`** - Returns the list of changes based on the provided `start_date`.
If no `start_date` is provided it returns the oldest set of recorded changes.
Changes for the last 4 weeks are retained. The items will be returned using
`processed_at` in ASC order (oldest first).

**NEW BEHAVIOR :**
The old `ledger` scope will only work on the old behavior system. As soon as you opt in to the new version, or when the sunset phase starts and you haven't explicitly opted out of the old behavior, the ledger scope will no longer work.
For more details, see our API documentation https://pennylane.readme.io/docs/2026-api-changes-guide for migration instructions.


> ℹ️
> This endpoint requires one of the following scopes: `ledger (DEPRECATED)`, `ledger_entries:readonly`, `ledger_entries:all`
- **`accounting-pp-cli external get-ledger-entry-lines`** - List ledger entry lines

> ℹ️
> This endpoint requires one of the following scopes: `ledger (DEPRECATED)`, `ledger_entries:readonly`, `ledger_entries:all`
- **`accounting-pp-cli external get-ledger-entry-lines-categories`** - List categories of a Ledger Entry line

**NEW BEHAVIOR :**
The old `ledger` scope will only work on the old behavior system. As soon as you opt in to the new version, or when the sunset phase starts and you haven't explicitly opted out of the old behavior, the ledger scope will no longer work.
For more details, see our API documentation https://pennylane.readme.io/docs/2026-api-changes-guide for migration instructions.


> ℹ️
> This endpoint requires one of the following scopes: `ledger (DEPRECATED)`, `ledger_entries:readonly`, `ledger_entries:all`
- **`accounting-pp-cli external get-ledger-entry-lines-lettered-ledger-entry-lines`** - List ledger entry lines lettered to a given ledger entry line

**DEPRECATED BEHAVIOR:**
The items rendered are sorted ordered by ascending `id`.

**NEW BEHAVIOR:**
The items rendered are sorted ordered by descending `id` by default.
A new sort param is available to customize the sorting behavior (see "sort" query parameter description).
The old `ledger` scope will only work on the old behavior system. As soon as you opt in to the new version, or when the sunset phase starts and you haven't explicitly opted out of the old behavior, the ledger scope will no longer work.
For more details, see our API documentation https://pennylane.readme.io/docs/2026-api-changes-guide for migration instructions.


> ℹ️
> This endpoint requires one of the following scopes: `ledger (DEPRECATED)`, `ledger_entries:readonly`, `ledger_entries:all`
- **`accounting-pp-cli external get-me`** - This endpoint returns information about the company and the user associated to the token.
- **`accounting-pp-cli external get-pa-registrations`** - Returns all PA (Plateforme Agrée) registrations for the company,
including activation status and exchange direction. Use this to determine whether the company has completed
PA onboarding. Records with a `null` `siret` represent the SIREN-level
(head office), other records represent establishments.


> ℹ️
> This endpoint requires the following scope: `pa_registrations:readonly`
- **`accounting-pp-cli external get-pro-account-mandate-migrations`** - This endpoint allows you to retrieve all mandate migration candidates
for your company. These are mandates that can be migrated to a Pro Account.

Requirements:
- Company must have a Pro Account (returns 404 if not)
- Company must have an enabled merchant profile (returns 403 if not)


> ℹ️
> This endpoint requires one of the following scopes: `customer_mandates:readonly`, `customer_mandates:all`
- **`accounting-pp-cli external get-pro-account-mandates`** - This endpoint allows you to retrieve all payment mandates associated
with your company's pro account.

Requirements:
- Company must have a Pro Account (returns 404 if not)
- Company must have an enabled merchant profile (returns 403 if not)


> ℹ️
> This endpoint requires one of the following scopes: `customer_mandates:readonly`, `customer_mandates:all`
- **`accounting-pp-cli external get-product`** - Retrieve a product

> ℹ️
> This endpoint requires one of the following scopes: `products:all`, `products:readonly`
- **`accounting-pp-cli external get-product-changes`** - Returns the list of changes based on the provided `start_date`.
If no `start_date` is provided it returns the oldest set of recorded changes.
Changes for the last 4 weeks are retained. The items will be returned using
`processed_at` in ASC order (oldest first).


> ℹ️
> This endpoint requires one of the following scopes: `products:all`, `products:readonly`
- **`accounting-pp-cli external get-products`** - List products

> ℹ️
> This endpoint requires one of the following scopes: `products:all`, `products:readonly`
- **`accounting-pp-cli external get-purchase-request`** - Retrieve a purchase request

> ℹ️
> This endpoint requires one of the following scopes: `purchase_requests:all`, `purchase_requests:readonly`
- **`accounting-pp-cli external get-purchase-requests`** - List purchase requests

> ℹ️
> This endpoint requires one of the following scopes: `purchase_requests:all`, `purchase_requests:readonly`
- **`accounting-pp-cli external get-quote`** - This endpoint retrieves a quote.

> ℹ️
> This endpoint requires one of the following scopes: `quotes:all`, `quotes:readonly`
- **`accounting-pp-cli external get-quote-appendices`** - List appendices of a quote

> ℹ️
> This endpoint requires one of the following scopes: `quotes:all`, `quotes:readonly`
- **`accounting-pp-cli external get-quote-changes`** - Returns the list of changes based on the provided `start_date`.
If no `start_date` is provided it returns the oldest set of recorded changes.
Changes for the last 4 weeks are retained. The items will be returned using
`processed_at` in ASC order (oldest first).


> ℹ️
> This endpoint requires one of the following scopes: `quotes:all`, `quotes:readonly`
- **`accounting-pp-cli external get-quote-invoice-line-sections`** - List invoice line sections for a quote

> ℹ️
> This endpoint requires one of the following scopes: `quotes:all`, `quotes:readonly`
- **`accounting-pp-cli external get-quote-invoice-lines`** - List invoice lines for a quote

> ℹ️
> This endpoint requires one of the following scopes: `quotes:all`, `quotes:readonly`
- **`accounting-pp-cli external get-sepa-mandate`** - This endpoint allows you to retrieve a specific SEPA mandate by ID

> ℹ️
> This endpoint requires one of the following scopes: `customer_mandates:all`, `customer_mandates:readonly`
- **`accounting-pp-cli external get-sepa-mandates`** - This endpoint allows you to retrieve all SEPA mandates associated with your company

> ℹ️
> This endpoint requires one of the following scopes: `customer_mandates:all`, `customer_mandates:readonly`
- **`accounting-pp-cli external get-supplier`** - This endpoint returns a supplier.

> ℹ️
> This endpoint requires one of the following scopes: `suppliers:all`, `suppliers:readonly`
- **`accounting-pp-cli external get-supplier-categories`** - List categories of a supplier

> ℹ️
> This endpoint requires one of the following scopes: `suppliers:readonly`, `suppliers:all`
- **`accounting-pp-cli external get-supplier-changes`** - Returns the list of changes based on the provided `start_date`.
If no `start_date` is provided it returns the oldest set of recorded changes.
Changes for the last 4 weeks are retained. The items will be returned using
`processed_at` in ASC order (oldest first).


> ℹ️
> This endpoint requires one of the following scopes: `suppliers:all`, `suppliers:readonly`
- **`accounting-pp-cli external get-supplier-invoice`** - This endpoint returns a supplier invoice.

> ℹ️
> This endpoint requires one of the following scopes: `supplier_invoices:all`, `supplier_invoices:readonly`
- **`accounting-pp-cli external get-supplier-invoice-categories`** - List categories of a supplier invoice

> ℹ️
> This endpoint requires one of the following scopes: `supplier_invoices:all`, `supplier_invoices:readonly`
- **`accounting-pp-cli external get-supplier-invoice-lines`** - List invoice lines for a supplier invoice

> ℹ️
> This endpoint requires one of the following scopes: `supplier_invoices:all`, `supplier_invoices:readonly`
- **`accounting-pp-cli external get-supplier-invoice-matched-transactions`** - List matched transactions for a supplier invoice

> ℹ️
> This endpoint requires one of the following scopes: `supplier_invoices:all`, `supplier_invoices:readonly`
- **`accounting-pp-cli external get-supplier-invoice-payments`** - List payments for a supplier invoice

> ℹ️
> This endpoint requires one of the following scopes: `supplier_invoices:all`, `supplier_invoices:readonly`
- **`accounting-pp-cli external get-supplier-invoices`** - This endpoint returns a list of supplier invoices.

> ℹ️
> This endpoint requires one of the following scopes: `supplier_invoices:all`, `supplier_invoices:readonly`
- **`accounting-pp-cli external get-supplier-invoices-changes`** - Returns the list of changes based on the provided `start_date`.
If no `start_date` is provided it returns the oldest set of recorded changes.
Changes for the last 4 weeks are retained. The items will be returned using
`processed_at` in ASC order (oldest first).


> ℹ️
> This endpoint requires one of the following scopes: `supplier_invoices:all`, `supplier_invoices:readonly`
- **`accounting-pp-cli external get-suppliers`** - List suppliers

> ℹ️
> This endpoint requires one of the following scopes: `suppliers:all`, `suppliers:readonly`
- **`accounting-pp-cli external get-transaction`** - Retrieve a transaction

> ℹ️
> This endpoint requires one of the following scopes: `transactions:readonly`, `transactions:all`
- **`accounting-pp-cli external get-transaction-categories`** - List categories of a bank transaction

> ℹ️
> This endpoint requires one of the following scopes: `transactions:readonly`, `transactions:all`
- **`accounting-pp-cli external get-transaction-changes`** - Returns the list of changes based on the provided `start_date`.
If no `start_date` is provided it returns the oldest set of recorded changes.
Changes for the last 4 weeks are retained. The items will be returned using
`processed_at` in ASC order (oldest first).


> ℹ️
> This endpoint requires one of the following scopes: `transactions:all`, `transactions:readonly`
- **`accounting-pp-cli external get-transaction-matched-invoices`** - List invoices matched to a bank transaction

> ℹ️
> This endpoint requires one of the following scopes: `transactions:readonly`, `transactions:all`
- **`accounting-pp-cli external get-transactions`** - List transactions

> ℹ️
> This endpoint requires one of the following scopes: `transactions:readonly`, `transactions:all`
- **`accounting-pp-cli external get-trial-balance`** - This endpoint returns the trial balance of the current company
for the given period.

**DEPRECATED BEHAVIOR:**
`page` and `per_page` params are **deprecated**. Please use `cursor` and `limit` for pagination instead.
For more details, see our API documentation https://pennylane.readme.io/docs/2026-api-changes-guide for migration instructions.

> ℹ️
> This endpoint requires the following scope: `trial_balance:readonly`
- **`accounting-pp-cli external get-webhook-subscription`** - This endpoint allows you to retrieve the webhook subscription for the authenticated
token. Note that the secret is not included in the response.
Each token (developer token or OAuth application) can only have one webhook subscription.
- **`accounting-pp-cli external import-customer-invoices`** - This endpoint allows you to import an invoice.

ℹ️ To ensure consistency, **we will apply validations on amounts in accordance with our rounding policy**. We allow a difference up to 1 cent per invoice_line between the total amounts and the sum of invoice lines. For further details, please refer to our [article](https://help.pennylane.com/fr/articles/61575-comprendre-l-arrondi-de-tva-d-un-produit) on rounding policy.


> ℹ️
> This endpoint requires the following scope: `customer_invoices:all`
- **`accounting-pp-cli external import-supplier-invoice`** - This endpoint allows you to import a supplier invoice with a file
attached.


> ℹ️
> This endpoint requires the following scope: `supplier_invoices:all`
- **`accounting-pp-cli external link-credit-note`** - Link a credit note to a customer invoice

> ℹ️
> This endpoint requires the following scope: `customer_invoices:all`
- **`accounting-pp-cli external list-commercial-documents`** - This endpoint lists commercial documents.

> ℹ️
> This endpoint requires one of the following scopes: `commercial_documents:all`, `commercial_documents:readonly`
- **`accounting-pp-cli external list-quotes`** - Lists quotes

> ℹ️
> This endpoint requires one of the following scopes: `quotes:all`, `quotes:readonly`
- **`accounting-pp-cli external mark-as-paid-customer-invoice`** - Mark a customer invoice as paid. No automatic reconciliation will
be done once the invoice is marked as paid


> ℹ️
> This endpoint requires the following scope: `customer_invoices:all`
- **`accounting-pp-cli external post-bank-account`** - Create a bank account

> ℹ️
> This endpoint requires the following scope: `bank_accounts:all`
- **`accounting-pp-cli external post-billing-subscriptions`** - This endpoint allows you to create a subscription. Pennylane will generate the customer invoice each month.
You can also link the subscription to a GoCardless mandate.


> ℹ️
> This endpoint requires the following scope: `billing_subscriptions:all`
- **`accounting-pp-cli external post-categories`** - Create a category

> ℹ️
> This endpoint requires the following scope: `categories:all`
- **`accounting-pp-cli external post-commercial-document-appendices`** - Upload a file that will be an appendix attached to a commercial document.

Note that this will not upload a file into the DMS (GED).


> ℹ️
> This endpoint requires the following scope: `commercial_documents:all`
- **`accounting-pp-cli external post-company-customer`** - This endpoint returns the created company customer.

> ℹ️
> This endpoint requires the following scope: `customers:all`
- **`accounting-pp-cli external post-customer-invoice-appendices`** - Upload a file that will be an appendix attached to a customer invoice.

Note that this will not upload a file into the DMS (GED).


> ℹ️
> This endpoint requires the following scope: `customer_invoices:all`
- **`accounting-pp-cli external post-customer-invoice-matched-transactions`** - This endpoint allows you to match a transaction to a customer invoice. It is not applicable for draft invoices.

You can match one transaction with one customer invoice at a time. To match multiple transactions to a customer invoice, you need to call this endpoint multiple times.
It's possible to match a transaction to multiple customer invoices too.


> ℹ️
> This endpoint requires the following scope: `customer_invoices:all`
- **`accounting-pp-cli external post-customer-invoices`** - This endpoint allows you to create a draft or finalized customer
invoice or credit note


> ℹ️
> This endpoint requires the following scope: `customer_invoices:all`
- **`accounting-pp-cli external post-file-attachments`** - Upload a file to attach to any resource that provides a `file_attachment_id`.

The maximum allowed file size is 100MB.
Note that this will not upload a file into the DMS (GED).


> ℹ️
> This endpoint requires the following scope: `file_attachments:all`
- **`accounting-pp-cli external post-gocardless-mandate-associations`** - This endpoint allows you to associate a GoCardless mandate to a customer.

> ℹ️
> This endpoint requires the following scope: `customer_mandates:all`
- **`accounting-pp-cli external post-gocardless-mandate-cancellations`** - Cancels a specific Gocardless mandate by ID. The mandate must be in a cancellable state, having one of the following statuses: `pending_submission`, `submitted` or `active`.

> ℹ️
> This endpoint requires the following scope: `customer_mandates:all`
- **`accounting-pp-cli external post-gocardless-mandate-mail-requests`** - This endpoint allows you to send an email request for a GoCardless mandate to a recipient.

> ℹ️
> This endpoint requires the following scope: `customer_mandates:all`
- **`accounting-pp-cli external post-individual-customer`** - This endpoint returns the created individual customer.

> ℹ️
> This endpoint requires the following scope: `customers:all`
- **`accounting-pp-cli external post-journals`** - Create a journal

**NEW BEHAVIOR :**
The old `ledger` scope will only work on the old behavior system. As soon as you opt in to the new version, or when the sunset phase starts and you haven't explicitly opted out of the old behavior, the ledger scope will no longer work.
For more details, see our API documentation https://pennylane.readme.io/docs/2026-api-changes-guide for migration instructions.


> ℹ️
> This endpoint requires one of the following scopes: `ledger (DEPRECATED)`, `journals:all`
- **`accounting-pp-cli external post-ledger-accounts`** - Create a ledger account

**NEW BEHAVIOR :**
The old `ledger` scope will only work on the old behavior system. As soon as you opt in to the new version, or when the sunset phase starts and you haven't explicitly opted out of the old behavior, the ledger scope will no longer work.
For more details, see our API documentation https://pennylane.readme.io/docs/2026-api-changes-guide for migration instructions.


> ℹ️
> This endpoint requires one of the following scopes: `ledger (DEPRECATED)`, `ledger_accounts:all`
- **`accounting-pp-cli external post-ledger-attachments`** - Upload a file to attach to a ledger entry. The maximum allowed file size is 100MB.
Note that this will not upload a file into the DMS (GED).

> ‼️
> This endpoint is **DEPRECATED**
> As an alternative, please use the [File Attachments: Upload a file](https://pennylane.readme.io/reference/postfileattachments#/) endpoint.

> ℹ️
> This endpoint requires the following scope: `ledger`
- **`accounting-pp-cli external post-ledger-entries`** - Create a ledger entry

**DEPRECATED BEHAVIOR:**
The old `ledger` scope will only work on the old behavior system. As soon as you opt in to the new version, or when the sunset phase starts and you haven't explicitly opted out of the old behavior, the ledger scope will no longer work.
For more details, see our API documentation https://pennylane.readme.io/docs/2026-api-changes-guide for migration instructions.


> ℹ️
> This endpoint requires one of the following scopes: `ledger (DEPRECATED)`, `ledger_entries:all`
- **`accounting-pp-cli external post-ledger-entry-lines-letter`** - This endpoint lets you letter ledger entry lines together. All
received entry lines will be lettered together. If a passed entry line is
already lettered, then the lettering will be applied to its associated lettered
entry lines as well.

> ℹ️
> This endpoint requires one of the following scopes: `ledger (DEPRECATED)`, `ledger_entries:all`
- **`accounting-pp-cli external post-pro-account-mandate-mail-requests`** - This endpoint allows you to send a mandate request for a Pro Account
SEPA Direct Debit mandate to a customer.

Requirements:
- Company must have a Pro Account (returns 404 if not)
- Company must have an enabled merchant profile (returns 403 if not)


> ℹ️
> This endpoint requires the following scope: `customer_mandates:all`
- **`accounting-pp-cli external post-pro-account-mandate-migrations`** - This endpoint allows you to migrate a mandate to a Pro Account.
Only mandates with status 'available' are eligible for migration.

**Requirements:**
- Company must have a Pro Account (returns 404 if not)
- Company must have an enabled merchant profile (returns 403 if not)


> ℹ️
> This endpoint requires the following scope: `customer_mandates:all`
- **`accounting-pp-cli external post-products`** - Create a product

> ℹ️
> This endpoint requires the following scope: `products:all`
- **`accounting-pp-cli external post-quote-appendices`** - Upload a file that will be an appendix attached to a quote.

Note that this will not upload a file into the DMS (GED).


> ℹ️
> This endpoint requires the following scope: `quotes:all`
- **`accounting-pp-cli external post-quotes`** - This endpoint allows you to create a quote

> ℹ️
> This endpoint requires the following scope: `quotes:all`
- **`accounting-pp-cli external post-sepa-mandates`** - This endpoint allows you to create a SEPA mandate to enable direct debit payments

> ℹ️
> This endpoint requires the following scope: `customer_mandates:all`
- **`accounting-pp-cli external post-supplier`** - This endpoint returns the created supplier.

> ℹ️
> This endpoint requires the following scope: `suppliers:all`
- **`accounting-pp-cli external post-supplier-invoice-linked-purchase-requests`** - This endpoint allows you to link a purchase request to a supplier invoice.

You can link one purchase request with one supplier invoice at a time. To link multiple purchase request to a supplier invoice, you need to call this endpoint multiple times.
It's possible to link a purchase request to multiple supplier invoices too.


> ℹ️
> This endpoint requires the following scope: `supplier_invoices:all`
- **`accounting-pp-cli external post-supplier-invoice-matched-transactions`** - This endpoint allows you to match a transaction to a supplier invoice.

You can match one transaction with one supplier invoice at a time. To match multiple transactions to a supplier invoice, you need to call this endpoint multiple times.
It's possible to match a transaction to multiple supplier invoices too.


> ℹ️
> This endpoint requires the following scope: `supplier_invoices:all`
- **`accounting-pp-cli external post-webhook-subscription`** - Creates a webhook subscription to receive real-time notifications for events occurring within your company or companies.

**Authentication & Scope**
- **Developer Token**: The subscription is scoped to the single company linked to the token.
- **OAuth Application Access Token**: The subscription covers **all companies** accessible by the OAuth application.

> ⚠️ **Limit**: Only one webhook subscription is allowed per OAuth Application or Developer Token.

**Secret**
. The secret will be auto-generated.

> 🔒 The secret is **only returned in the creation response** and cannot be retrieved afterwards. Make sure to store it securely.
- **`accounting-pp-cli external put-billing-subscriptions`** - Update a billing subscription

> ℹ️
> This endpoint requires the following scope: `billing_subscriptions:all`
- **`accounting-pp-cli external put-company-customer`** - This endpoint returns the updated company customer.

> ℹ️
> This endpoint requires the following scope: `customers:all`
- **`accounting-pp-cli external put-customer-categories`** - Update the categories of a customer. You can pass categories that don't belong to the same category group. The sum of categories of a same group must equal `1`. In the following example, the two first categories belong to the same category group A, the sum of the weights is `1`. The third category belongs to a category group B, its weight is `1`.
```
[
  { "id": 59, "weight": "0.5" }, // category group A
  { "id": 33, "weight": "0.5" }, // category group A
  { "id": 65, "weight": "1" }    // category group B
]
```


> ℹ️
> This endpoint requires the following scope: `customers:all`
- **`accounting-pp-cli external put-customer-invoice-categories`** - This endpoint is not applicable for draft invoices.
Update the categories of a customer invoice. You can pass categories that don't belong to the same category group. The sum of categories of a same group must equal `1`. In the following example, the two first categories belong to the same category group A, the sum of the weights is `1`. The third category belongs to a category group B, its weight is `1`.
```
[
  { "id": 59, "weight": "0.5" }, // category group A
  { "id": 33, "weight": "0.5" }, // category group A
  { "id": 65, "weight": "1" } // category group B
]
```


> ℹ️
> This endpoint requires the following scope: `customer_invoices:all`
- **`accounting-pp-cli external put-individual-customer`** - This endpoint returns the updated individual customer.

> ℹ️
> This endpoint requires the following scope: `customers:all`
- **`accounting-pp-cli external put-ledger-entries`** - Update a ledger entry

**NEW BEHAVIOR :**
The old `ledger` scope will only work on the old behavior system. As soon as you opt in to the new version, or when the sunset phase starts and you haven't explicitly opted out of the old behavior, the ledger scope will no longer work.
For more details, see our API documentation https://pennylane.readme.io/docs/2026-api-changes-guide for migration instructions.


> ℹ️
> This endpoint requires one of the following scopes: `ledger (DEPRECATED)`, `ledger_entries:all`
- **`accounting-pp-cli external put-ledger-entry-lines-categories`** - This endpoint replaces already existing categories on the Ledger Entry line with new values.
If an empty array of categories_ids is provided, it will remove all categories from the Ledger Entry line.

**NEW BEHAVIOR :**
The old `ledger` scope will only work on the old behavior system. As soon as you opt in to the new version, or when the sunset phase starts and you haven't explicitly opted out of the old behavior, the ledger scope will no longer work.
For more details, see our API documentation https://pennylane.readme.io/docs/2026-api-changes-guide for migration instructions.


> ℹ️
> This endpoint requires one of the following scopes: `ledger (DEPRECATED)`, `ledger_entries:all`
- **`accounting-pp-cli external put-product`** - Update a product

> ℹ️
> This endpoint requires the following scope: `products:all`
- **`accounting-pp-cli external put-sepa-mandate`** - This endpoint allows you to update an existing SEPA mandate

> ℹ️
> This endpoint requires the following scope: `customer_mandates:all`
- **`accounting-pp-cli external put-supplier`** - This endpoint returns the updated supplier.

> ℹ️
> This endpoint requires the following scope: `suppliers:all`
- **`accounting-pp-cli external put-supplier-categories`** - Update the categories of a supplier. You can pass categories that don't belong to the same category group. The sum of categories of a same group must equal `1`. In the following example, the two first categories belong to the same category group A, the sum of the weights is `1`. The third category belongs to a category group B, its weight is `1`.
```
[
  { "id": 59, "weight": "0.5" }, // category group A
  { "id": 33, "weight": "0.5" }, // category group A
  { "id": 65, "weight": "1" }    // category group B
]
```


> ℹ️
> This endpoint requires the following scope: `suppliers:all`
- **`accounting-pp-cli external put-supplier-invoice`** - This endpoint allows you to update a supplier invoice.

> ℹ️
> This endpoint requires the following scope: `supplier_invoices:all`
- **`accounting-pp-cli external put-supplier-invoice-categories`** - Update the categories of a supplier invoice. You can pass categories that don't belong to the same category group. The sum of categories of a same group must equal `1`. In the following example, the two first categories belong to the same category group A, the sum of the weights is `1`. The third category belongs to a category group B, its weight is `1`.
```
[
  { "id": 59, "weight": "0.5" }, // category group A
  { "id": 33, "weight": "0.5" }, // category group A
  { "id": 65, "weight": "1" } // category group B
]
```


> ℹ️
> This endpoint requires the following scope: `supplier_invoices:all`
- **`accounting-pp-cli external put-supplier-invoice-einvoice-status`** - Applies an electronic invoicing lifecycle transition: dispute, refuse, or undispute (approved).
Dispute and refuse require a reason.


> ℹ️
> This endpoint requires the following scope: `supplier_invoices:all`
- **`accounting-pp-cli external put-transaction-categories`** - Update the categories of a transaction. You can pass categories that don't belong to the same category group. The sum of categories of a same group must equal `1`. In the following example, the two first categories belong to the same category group A, the sum of the weights is `1`. The third category belongs to a category group B, its weight is `1`.
```
[
  { "id": 59, "weight": "0.5" }, // category group A
  { "id": 33, "weight": "0.5" }, // category group A
  { "id": 65, "weight": "1" } // category group B
]
```


> ℹ️
> This endpoint requires the following scope: `transactions:all`
- **`accounting-pp-cli external put-webhook-subscription`** - This endpoint allows you to update the webhook subscription for the authenticated
token. Each token (developer token or OAuth application) can only have one webhook subscription.
- **`accounting-pp-cli external send-by-email-customer-invoice`** - This endpoint allows you to send a finalized, imported customer invoice or credit note
by email to your customer. This requires that the PDF file for that document
has been generated (this process can take a few minutes), so if you just created
the invoice in our system, we may return a 409 error. You should
retry the request in a few minutes - if you receive a 204 response, that means
that the email is on its way. For more information about email sending, please
read [this guide](https://pennylane.readme.io/v2.0/docs/sending-documents-by-email).


> ℹ️
> This endpoint requires the following scope: `customer_invoices:all`
- **`accounting-pp-cli external send-by-email-quote`** - This endpoint allows you to send a quote by email to your customer.
This requires that the PDF file for that document has been generated
(this process can take a few minutes), so if you just created
the quote in our system, we may return a 409 error. You should
retry the request in a few minutes - if you receive a 204 response, that means
that the email is on its way. For more information about email sending, please
read \[this guide\](https://pennylane.readme.io/v2.0/docs/sending-documents-by-email).


> ℹ️
> This endpoint requires the following scope: `quotes:all`
- **`accounting-pp-cli external update-category`** - This endpoint updates a category.

> ℹ️
> This endpoint requires the following scope: `categories:all`
- **`accounting-pp-cli external update-customer-invoice`** - Update a customer invoice

> ℹ️
> This endpoint requires the following scope: `customer_invoices:all`
- **`accounting-pp-cli external update-imported-customer-invoice`** - Update an imported customer invoice or credit note. It is not
applicable for draft invoices.


> ℹ️
> This endpoint requires the following scope: `customer_invoices:all`
- **`accounting-pp-cli external update-ledger-account`** - Update a ledger account

> ℹ️
> This endpoint requires the following scope: `ledger_accounts:all`
- **`accounting-pp-cli external update-quote`** - This endpoint allows you to update a quote

> ℹ️
> This endpoint requires the following scope: `quotes:all`
- **`accounting-pp-cli external update-status-quote`** - This endpoint allows you to update the status of a quote

> ℹ️
> This endpoint requires the following scope: `quotes:all`
- **`accounting-pp-cli external update-supplier-invoice-payment-status`** - This endpoint allows you to update the payment status of a supplier
invoice.


> ℹ️
> This endpoint requires the following scope: `supplier_invoices:all`
- **`accounting-pp-cli external update-transaction`** - This endpoint returns the updated transaction.

> ℹ️
> This endpoint requires the following scope: `transactions:all`
- **`accounting-pp-cli external validate-accounting-supplier-invoice`** - Turn the supplier invoice into a Complete state.

> ℹ️
> This endpoint requires the following scope: `supplier_invoices:all`


## Cookbook

Real-world recipes using verified flag names.

### First run — check connectivity and credentials

```bash
accounting-pp-cli doctor
```

### Sync all data locally (offline mode)

```bash
accounting-pp-cli sync
```

### AR aging report — monthly receivables review

```bash
accounting-pp-cli ar aging --buckets 0,30,60,90 --json
```

### Cash runway — when will we run out of cash?

```bash
accounting-pp-cli cash runway --horizon 90 --json
```

### DSO — identify slow-paying clients

```bash
accounting-pp-cli cash dso --rolling 90 --json
```

### VAT preview for Q1 declaration

```bash
accounting-pp-cli vat preview --period 2026-Q1 --json
```

### Detect payment anomalies (duplicates, orphan round amounts)

```bash
accounting-pp-cli audit anomalies --sigma 2 --json --agent
```

### Top clients by revenue year-to-date

```bash
accounting-pp-cli clients rank --by revenue --ytd --json
```

### Supplier payment schedule — next 60 days

```bash
accounting-pp-cli ap schedule --horizon 60 --json
```

### Overdue receivables to follow up

```bash
accounting-pp-cli ar remind --overdue-days 30 --dry-run --json
```

### Year-end checklist

```bash
accounting-pp-cli yearend check --fiscal-year 2025 --json
```

### Validate a FEC file before DGFiP submission

```bash
accounting-pp-cli fec validate --file FEC2025.txt --json
```

### Bulk-create invoices from CSV (dry-run first)

```bash
accounting-pp-cli invoice bulk-create --file invoices.csv --dry-run
accounting-pp-cli invoice bulk-create --file invoices.csv
```

### Check recurring invoice drift (last 6 months, 5% tolerance)

```bash
accounting-pp-cli invoice check-recurring --tolerance 5 --months 6 --json
```

### Find the right command for any capability

```bash
accounting-pp-cli which "cancel a supplier invoice"
```

### Pipe AR aging to jq for custom analysis

```bash
accounting-pp-cli ar aging --json | jq '.results[] | select(.bucket == "90+")'
```

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
accounting-pp-cli external company-fiscal-years

# JSON for scripting and agents
accounting-pp-cli external company-fiscal-years --json

# Filter to specific fields
accounting-pp-cli external company-fiscal-years --json --select id,name,status

# Dry run — show the request without sending
accounting-pp-cli external company-fiscal-years --dry-run

# Agent mode — JSON + compact + no prompts in one flag
accounting-pp-cli external company-fiscal-years --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-accounting -g
```

Then invoke `/pp-accounting <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add accounting accounting-pp-mcp -e ACCOUNTING_OAUTH2=<your-token>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/accounting-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `ACCOUNTING_OAUTH2` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "accounting": {
      "command": "accounting-pp-mcp",
      "env": {
        "ACCOUNTING_OAUTH2": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
accounting-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/accounting-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `ACCOUNTING_OAUTH2` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `accounting-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $ACCOUNTING_OAUTH2`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
