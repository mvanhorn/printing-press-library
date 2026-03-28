---
title: "Feature Parity Audit: Dub CLI"
type: feat
status: active
date: 2026-03-27
phase: "0.6"
api: "dub"
---

# Feature Parity Audit: Dub CLI

## Competitor Analysis

Only one CLI competitor exists: **sujjeee/dubco** (24 stars, TypeScript, last commit Aug 2024).

Since there's no strong CLI competitor, we also compare against the **Dub web dashboard** to identify what users already can do via UI that a CLI must match.

## Feature Matrix

| Feature | dubco CLI (24★) | Dub Dashboard | Ours | Classification |
|---------|----------------|---------------|------|----------------|
| **Links** | | | | |
| Create short link | ✅ (interactive) | ✅ | ✅ | TABLE STAKES |
| Create link with flags (non-interactive) | ❌ | N/A | ✅ | TABLE STAKES |
| List links | ❌ | ✅ | ✅ | TABLE STAKES |
| Get link details | ❌ | ✅ | ✅ | TABLE STAKES |
| Update link | ❌ | ✅ | ✅ | TABLE STAKES |
| Delete link | ❌ | ✅ | ✅ | TABLE STAKES |
| Bulk create links | ❌ | ✅ (CSV import) | ✅ | TABLE STAKES |
| Bulk update links | ❌ | ❌ | ✅ | NICE-TO-HAVE |
| Bulk delete links | ❌ | ✅ (select+delete) | ✅ | TABLE STAKES |
| Search links | ❌ | ✅ | ✅ | TABLE STAKES |
| Filter by tag/domain/folder | ❌ | ✅ | ✅ | TABLE STAKES |
| Upsert link | ❌ | ❌ | ✅ | NICE-TO-HAVE |
| **Domains** | | | | |
| Add custom domain | ❌ | ✅ | ✅ | TABLE STAKES |
| List domains | ❌ | ✅ | ✅ | TABLE STAKES |
| Verify domain status | ❌ | ✅ | ✅ | TABLE STAKES |
| Delete domain | ❌ | ✅ | ✅ | TABLE STAKES |
| Register domain | ❌ | ✅ | ✅ | NICE-TO-HAVE |
| **Tags** | | | | |
| Create/list/update/delete tags | ❌ | ✅ | ✅ | TABLE STAKES |
| **Folders** | | | | |
| Create/list/update/delete folders | ❌ | ✅ | ✅ | TABLE STAKES |
| **Analytics** | | | | |
| View click analytics | ❌ | ✅ | ✅ | TABLE STAKES |
| Filter by geo/device/browser/referer | ❌ | ✅ | ✅ | TABLE STAKES |
| Filter by time range | ❌ | ✅ | ✅ | TABLE STAKES |
| Export analytics (CSV) | ❌ | ✅ (web export) | ✅ | TABLE STAKES |
| **Events** | | | | |
| View recent events | ❌ | ✅ | ✅ | TABLE STAKES |
| **Tracking** | | | | |
| Track lead | ❌ | SDK | ✅ | NICE-TO-HAVE |
| Track sale | ❌ | SDK | ✅ | NICE-TO-HAVE |
| **Partners/Affiliates** | | | | |
| List partners | ❌ | ✅ | ✅ | TABLE STAKES |
| Partner analytics | ❌ | ✅ | ✅ | TABLE STAKES |
| Manage commissions | ❌ | ✅ | ✅ | NICE-TO-HAVE |
| View payouts | ❌ | ✅ | ✅ | NICE-TO-HAVE |
| **Customers** | | | | |
| List/get/update/delete customers | ❌ | ✅ | ✅ | TABLE STAKES |
| **QR Codes** | | | | |
| Generate QR code | ❌ | ✅ | ✅ | NICE-TO-HAVE |
| **Auth & Config** | | | | |
| API key auth | ✅ | N/A | ✅ | TABLE STAKES |
| Doctor/health check | ❌ | N/A | ✅ | TABLE STAKES |
| --json output | ❌ | N/A | ✅ | TABLE STAKES |
| --dry-run | ❌ | N/A | ✅ | TABLE STAKES |
| --stdin for complex body | ❌ | N/A | ✅ | TABLE STAKES |

## Classification Summary

- **TABLE STAKES:** 26 features (everything the dashboard offers + CLI basics like --json, doctor, search)
- **NICE-TO-HAVE:** 7 features (domain registration, bulk update, upsert, tracking, commissions, payouts, QR)
- **ANTI-SCOPE:**
  - Full TUI mode: 0% of Dub users have a CLI, let alone a TUI. No competitor offers one.
  - Interactive link builder: dubco does this but it's not what power users want. Flags > prompts.
  - Webhook receiver: Would need a server. Out of scope for a CLI.

## Table Stakes for Phase 4

All 26 TABLE STAKES features become Phase 4 Priority 1 work items. The generator will produce most CRUD commands automatically. The workflow commands (snapshot, export, import, tail, compare, stale, health) add the data-layer value.
