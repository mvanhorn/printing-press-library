# Generator upgrade to 4.35.1

The reopened PR failed the new minimum-generator check because its base was generated with 4.34.2 and upstream now requires at least 4.35.0.

Installed and ran actual Printing Press 4.35.1 generation using the existing researched contract and developer-tools category. Reconciled the fresh framework with the reviewed CLI through regen-merge using a separately generated 4.34.2 baseline. All 16 hand-authored Go files, five local snapshot adapters, the reviewed mutex-safe tier clone, source-filtered search, selected HTTP settings, and optional SEC contact desktop configuration were preserved. Creator identity and release accounting were preserved. No feature scope changed.

The new framework refreshes credential/configuration and delivery code and their tests. A duplicate import introduced during merge was removed; the intake-lock example now explicitly requests JSON, matching its machine-readable preview use.

Validation: full unit suite PASS; go vet PASS; mock verification 49/49; verify-skill PASS; publish validate PASS including reachable govulncheck. Full live matrix: 186/186 executed checks PASS, zero failures, 136 skipped/unverified retained separately. No credentials or raw live transcripts are included.
