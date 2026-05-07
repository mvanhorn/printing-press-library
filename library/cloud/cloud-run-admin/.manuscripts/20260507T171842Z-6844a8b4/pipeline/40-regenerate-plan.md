---
title: "cloud-run-admin CLI Pipeline - Phase 3: Regenerate"
type: feat
status: seed
pipeline_phase: regenerate
pipeline_api: cloud-run-admin
date: 2026-05-07
---

# Phase Goal

Merge the enrichments into the source spec and regenerate the CLI without losing quality.

## Context

- Pipeline directory: /Users/cathrynlavery/printing-press/.runstate/mogadishu-36cf6133/runs/20260507T171842Z-6844a8b4/pipeline
- Output directory: /Users/cathrynlavery/printing-press/.runstate/mogadishu-36cf6133/runs/20260507T171842Z-6844a8b4/working/cloud-run-admin-pp-cli
- Spec URL: https://raw.githubusercontent.com/APIs-guru/openapi-directory/main/APIs/cloud-run-admin.com/v1/openapi.yaml
- Spec source: apis-guru (unverified, needs fetch validation)

## What This Phase Must Produce

- Re-generated CLI in /Users/cathrynlavery/printing-press/.runstate/mogadishu-36cf6133/runs/20260507T171842Z-6844a8b4/working/cloud-run-admin-pp-cli using the merged overlay
- Merged spec artifact suitable for regeneration
- All seven quality gates still passing after regeneration

## Prior Phase Outputs

- overlay.yaml from enrich in /Users/cathrynlavery/printing-press/.runstate/mogadishu-36cf6133/runs/20260507T171842Z-6844a8b4/pipeline
- Original scaffolded CLI in /Users/cathrynlavery/printing-press/.runstate/mogadishu-36cf6133/runs/20260507T171842Z-6844a8b4/working/cloud-run-admin-pp-cli

## Codebase Pointers

- Overlay merge implementation: internal/pipeline/merge.go
- MergeOverlay function in internal/pipeline/merge.go
- Generator entrypoint: printing-press generate
