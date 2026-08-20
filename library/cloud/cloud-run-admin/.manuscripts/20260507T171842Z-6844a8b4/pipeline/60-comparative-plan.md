---
title: "cloud-run-admin CLI Pipeline - Comparative Analysis"
type: feat
status: seed
pipeline_phase: comparative
pipeline_api: cloud-run-admin
date: 2026-05-07
---

# Phase Goal

Score the generated cloud-run-admin CLI against discovered alternatives on 6 dimensions.

## Context

- Pipeline directory: ~/printing-press/.runstate/mogadishu-36cf6133/runs/20260507T171842Z-6844a8b4/pipeline
- Output directory: ~/printing-press/.runstate/mogadishu-36cf6133/runs/20260507T171842Z-6844a8b4/working/cloud-run-admin-pp-cli
- Spec URL: https://api.apis.guru/v2/specs/googleapis.com/run/v2/openapi.yaml
- Spec source: catalog entry google-cloud-run (validated via apis.guru googleapis.com/run/v2)

## What This Phase Must Produce

- comparative-analysis.md in ~/printing-press/.runstate/mogadishu-36cf6133/runs/20260507T171842Z-6844a8b4/pipeline with:
  - Score table (our CLI vs each alternative, 100 points max)
  - Gap summary: what we're missing
  - Advantage summary: what we have that others don't
  - Ship recommendation: ship, ship-with-gaps, or hold

## Scoring Dimensions (100 points max)

| Dimension | Points | How Measured |
|-----------|--------|-------------|
| Breadth | 20 | Command count ratio vs best alternative |
| Install Friction | 20 | Go binary = 20, clone+build = 15, runtime = 10 |
| Auth UX | 15 | env var + config = 15, env only = 10, manual = 5 |
| Output Formats | 15 | 5 per format (JSON, table, plain) |
| Agent Friendliness | 15 | --json (5) + --dry-run (5) + non-interactive (5) |
| Freshness | 15 | <30d = 15, <90d = 10, <1yr = 5, >1yr = 0 |

## Prior Phase Outputs

- research.json from research phase
- dogfood-results.json from review phase
- Working CLI binary in ~/printing-press/.runstate/mogadishu-36cf6133/runs/20260507T171842Z-6844a8b4/working/cloud-run-admin-pp-cli

## Codebase Pointers

- Comparative logic: internal/pipeline/comparative.go
- Research results: ~/printing-press/.runstate/mogadishu-36cf6133/runs/20260507T171842Z-6844a8b4/pipeline/research.json
- Dogfood results: ~/printing-press/.runstate/mogadishu-36cf6133/runs/20260507T171842Z-6844a8b4/pipeline/dogfood-results.json
