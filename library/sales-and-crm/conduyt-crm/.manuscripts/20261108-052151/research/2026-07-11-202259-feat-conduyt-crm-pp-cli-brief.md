# Conduyt CRM CLI Brief (reprint 2026-07-11)

## API Identity
- Domain: conduyt.app — first-party AI-native CRM (Next.js/Prisma/Neon), 814 catalog endpoints / 508 tenant-facing OpenAPI paths across ~70 domains.
- Users: (1) Paul — owner/admin, BDA ops: imports, drips, SMS delivery forensics, dialer reports; (2) headless AI agents (the #1 persona — "a CRM for you, your agents, and your AI team") driving contacts/deals/automations via Bearer keys; (3) ops scripts (n8n, cron).
- Data profile: contacts (200k+ at BDA), deals, pipelines, automations + execution logs, drip ledgers, import jobs, messages, invoices. High-gravity: contacts, deals, imports, automations, SMS delivery outcomes.

## Reachability Risk
- None. First-party API, health 200, Bearer key on hand (read-only smoke approved). Rate limits per-route (30-60/min typical).

## Top Workflows
1. Bulk lead import with pre-flight safety: preflight → import (verifyLineType/reEnrollMode) → watch job → verify outcomes. THE Kloudi-incident workflow.
2. SMS delivery forensics: reports/sms-delivery — who entered, what the provider did (rejected-invalid/litigator/DNC), WHY (line_type vs our landline_skipped/no_phone_skipped skips).
3. Phone line-type verification: verify-line-type loop-until-done over a smart list; stamps line type/carrier/valid on contacts (tenant-paid Twilio).
4. Contact/deal CRUD + search + bulk ops for agents (table stakes, 550+ endpoints).
5. Automation ops: validate/import/dry-run/publish workflows, step-logs triage.

## Table Stakes
- Full endpoint mirror (prior CLI: 418 command files). Sync/SQL/FTS local store. --json/--select/--dry-run/typed exits. Doctor. Agent-context.

## Data Layer
- Primary entities: contacts, deals, companies, tasks, automations, import_jobs, messages.
- Sync cursor: updatedAt per resource. FTS: contacts/deals/notes.

## User Vision
- REPRINT PURPOSE (Paul, 2026-07-11): tri-surface parity for the Kloudi verification builds — POST /contacts/verify-line-type (batched loop-until-done), POST /imports/preflight (criticalWarnings: phone-less-for-SMS, heavy create-only skips, double-mapped columns), imports.verifyLineType create-param, GET /reports/sms-delivery (incl. landline_skipped/no_phone_skipped). These must be FIRST-CLASS commands with real UX (loop-until-done driver, human-readable warning rendering), not endpoint mirrors. Keep semantics consistent with conduyt-mcp@4.10.0 tool names/params (conduyt_verify_line_type, conduyt_import_preflight, conduyt_sms_delivery_report). Keep binary names conduyt-crm-pp-cli / conduyt-crm-pp-mcp.

## Product Thesis
- Name: conduyt-crm-pp-cli ("Conduyt CRM")
- Why: the official CLI surface of a CRM whose differentiator IS AI-accessibility; agents get offline search + bounded output + the same verification/forensics tools the app and MCP ship.

## Build Priorities
1. The four verification/forensics surfaces as first-class commands (loop driver for verify; preflight warning renderer; delivery report with reason breakdown).
2. Full endpoint mirror from the 508-path spec (absorbs prior CLI coverage).
3. Local store + sync/search/SQL (carried forward).
4. MCP: Cloudflare pattern (stdio+http, code orchestration, hidden endpoint tools) — 508 paths ≫ 50-tool threshold.
