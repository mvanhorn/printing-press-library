# Changelog

This file is maintained by printing-press-library release automation. Do not hand-edit release sections in normal PRs.

## 2026.11.1 - 2026-11-05

- fix(concur): harden browser fallbacks; add receipts attach fallback and per-report expense sync (#2428).

## 2026.10.4 - 2026-10-19

- fix(concur): evict cached reads after browser-driven writes; read exceptions live (#2338).

## 2026.10.3 - 2026-10-19

- fix(concur): refuse to save an already-expired session in auth login (#2340).

## 2026.10.2 - 2026-10-18

- feat(concur): inspect and fix itemization lines; attach fallbacks to chrome://inspect Chrome (#2336).

## 2026.10.1 - 2026-10-16

- fix(concur): read regional press-auth captures; infer policyId before the browser fallback (#2322).

## 2026.9.6 - 2026-09-29

- fix+feat(concur): resolve custom-named Chrome profile resolution, add expenses list command (#2078).

## 2026.9.5 - 2026-09-25

- fix(concur): add browser-automation fallbacks for expenses-update and reports-submit backend defects (#2055).

## 2026.9.4 - 2026-09-16

- fix+feat(concur): correct expenses-create body shape and add browser-automation fallback for its live 404 defect (#2003).

## 2026.9.3 - 2026-09-11

- fix: fail closed on ambiguous fleet write retries (#1978).

## 2026.9.2 - 2026-09-06

- fix(concur): leftover from #1936 — port-strip host trust for browser fallback (#1943).

## 2026.9.1 - 2026-09-05

- fix(concur): correct live API shape mismatches in expenses apply-rules, sync, and reports list (#1940).

## 2026.8.2 - 2026-08-30

- fix: bump github.com/enetx/http to v1.0.29 across 68 modules (#1865).

## 2026.8.1 - 2026-08-21

- Baseline release metadata added for this published CLI.
