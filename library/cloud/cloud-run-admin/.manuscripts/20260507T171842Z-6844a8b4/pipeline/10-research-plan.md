---
title: "cloud-run-admin CLI Pipeline - Research: Discover Alternatives"
type: feat
status: seed
pipeline_phase: research
pipeline_api: cloud-run-admin
date: 2026-05-07
---

# Phase Goal

Discover existing CLI tools for the cloud-run-admin API and assess whether generating a new one adds value.

## Context

- Pipeline directory: ~/printing-press/.runstate/mogadishu-36cf6133/runs/20260507T171842Z-6844a8b4/pipeline
- Output directory: ~/printing-press/.runstate/mogadishu-36cf6133/runs/20260507T171842Z-6844a8b4/working/cloud-run-admin-pp-cli
- Spec URL: https://api.apis.guru/v2/specs/googleapis.com/run/v2/openapi.yaml
- Spec source: catalog entry google-cloud-run (validated via apis.guru googleapis.com/run/v2)

## What This Phase Must Produce

- research.json in ~/printing-press/.runstate/mogadishu-36cf6133/runs/20260507T171842Z-6844a8b4/pipeline with:
  - List of discovered alternative CLIs (name, URL, language, stars)
  - Novelty score (1-10)
  - Recommendation: proceed, proceed-with-gaps, or skip
  - Gap analysis: what alternatives miss
  - Pattern analysis: what alternatives do well

## Steps

1. Check catalog/cloud-run-admin.yaml for known_alternatives field
2. Search GitHub for "cloud-run-admin cli" repos sorted by stars
3. Deduplicate and score alternatives
4. If novelty score <= 3, flag: "Official CLI exists - consider whether this CLI adds value"
5. Write research.json

## Prior Phase Outputs

- Validated spec URL from preflight

## Codebase Pointers

- Research logic: internal/pipeline/research.go
- Catalog entries: catalog/
- Known specs registry: internal/pipeline/discover.go
