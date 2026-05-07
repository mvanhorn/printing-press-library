---
title: "cloud-run-admin CLI Pipeline - Phase 1: Scaffold"
type: feat
status: seed
pipeline_phase: scaffold
pipeline_api: cloud-run-admin
date: 2026-05-07
---

# Phase Goal

Generate the first working cloud-run-admin CLI from the validated OpenAPI spec.

## Context

- Pipeline directory: ~/printing-press/.runstate/mogadishu-36cf6133/runs/20260507T171842Z-6844a8b4/pipeline
- Output directory: ~/printing-press/.runstate/mogadishu-36cf6133/runs/20260507T171842Z-6844a8b4/working/cloud-run-admin-pp-cli
- Spec URL: https://api.apis.guru/v2/specs/googleapis.com/run/v2/openapi.yaml
- Spec source: catalog entry google-cloud-run (validated via apis.guru googleapis.com/run/v2)

## What This Phase Must Produce

- Generated CLI source tree in ~/printing-press/.runstate/mogadishu-36cf6133/runs/20260507T171842Z-6844a8b4/working/cloud-run-admin-pp-cli
- All seven generator quality gates passing
- Working CLI binary for cloud-run-admin

## Prior Phase Outputs

- conventions.json from preflight in ~/printing-press/.runstate/mogadishu-36cf6133/runs/20260507T171842Z-6844a8b4/pipeline
- Validated spec URL and downloaded spec source for generation

## Codebase Pointers

- Generator entrypoint: printing-press generate --spec <url> --output <dir>
- Generator implementation: internal/generator/
- Quality gate logic in the generator flow under internal/generator/
