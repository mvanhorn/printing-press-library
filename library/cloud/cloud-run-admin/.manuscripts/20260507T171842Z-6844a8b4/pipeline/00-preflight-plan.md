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

- Pipeline directory: /Users/cathrynlavery/printing-press/.runstate/mogadishu-36cf6133/runs/20260507T171842Z-6844a8b4/pipeline
- Output directory: /Users/cathrynlavery/printing-press/.runstate/mogadishu-36cf6133/runs/20260507T171842Z-6844a8b4/working/cloud-run-admin-pp-cli
- Spec URL: https://raw.githubusercontent.com/APIs-guru/openapi-directory/main/APIs/cloud-run-admin.com/v1/openapi.yaml
- Spec source: apis-guru (unverified, needs fetch validation)

## What This Phase Must Produce

- Verified Go environment for the pipeline run
- Verified printing-press binary for local generation work
- Downloaded and validated OpenAPI spec for cloud-run-admin
- conventions.json in /Users/cathrynlavery/printing-press/.runstate/mogadishu-36cf6133/runs/20260507T171842Z-6844a8b4/pipeline

## Prior Phase Outputs

None.

## Codebase Pointers

- Build entrypoint: go build ./cmd/printing-press
- OpenAPI parsing: internal/openapi/parser.go
- Pipeline discovery flow: internal/pipeline/discover.go
