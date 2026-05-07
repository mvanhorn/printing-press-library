---
title: "cloud-run-admin CLI Pipeline - Phase 0: Preflight"
type: feat
status: seed
pipeline_phase: preflight
pipeline_api: cloud-run-admin
date: 2026-05-07
---

# Phase Goal

Verify the local environment and source inputs needed to run the cloud-run-admin CLI pipeline.

## Context

- Pipeline directory: ~/printing-press/.runstate/mogadishu-36cf6133/runs/20260507T171842Z-6844a8b4/pipeline
- Output directory: ~/printing-press/.runstate/mogadishu-36cf6133/runs/20260507T171842Z-6844a8b4/working/cloud-run-admin-pp-cli
- Spec URL: https://api.apis.guru/v2/specs/googleapis.com/run/v2/openapi.yaml
- Spec source: catalog entry google-cloud-run (validated via apis.guru googleapis.com/run/v2)

## What This Phase Must Produce

- Verified Go environment for the pipeline run
- Verified printing-press binary for local generation work
- Downloaded and validated OpenAPI spec for cloud-run-admin
- conventions.json in ~/printing-press/.runstate/mogadishu-36cf6133/runs/20260507T171842Z-6844a8b4/pipeline

## Prior Phase Outputs

None.

## Codebase Pointers

- Build entrypoint: go build ./cmd/printing-press
- OpenAPI parsing: internal/openapi/parser.go
- Pipeline discovery flow: internal/pipeline/discover.go
