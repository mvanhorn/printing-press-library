The run-local Printing Press checker patch accepts a truthful checkout install block only for the exact bare module `<api_name>-pp-cli`. Qualified modules still require the existing canonical public installation block. Exact equality, delimiter checks, malformed-module failures, and all existing canonical checks remain active. No product gate was skipped.

Upstream: `github.com/mvanhorn/cli-printing-press/v4@v4.32.5`, tag commit `7298ce9b1198c6689ae9a850e75b2ca2043c044a`.

Binary: `/Users/zjsng/printing-press/.runstate/zjsng-a9b2d4f4/runs/20260927-212939-86ef661f/toolchain/bin/cli-printing-press`

Binary SHA-256: `2461433c103dfb590604bd3f41a2d2b40a6c2796e65b616c3981acbba395b9cb`

Source: `/Users/zjsng/printing-press/toolchain-fixes/20260927-212939-86ef661f/cli-printing-press-v4.32.5-local-install`

Patch: `/Users/zjsng/printing-press/.runstate/zjsng-a9b2d4f4/runs/20260927-212939-86ef661f/toolchain/local-install-checker.patch`

Patch SHA-256: `32c5f401eebb8d72f477156aea2a72468ca1109e0b48c2549bedc81fdd172552`

Focused tests pass: 20 new local/published/module-evidence cases and 6 existing canonical subprocess regressions. The same truthful local fixture fails the installed upstream checker (exit 1) and passes the patched checker (exit 0). Qualified-module checkout instructions and malformed module evidence remain failures.

The full upstream suite was attempted and exited 1: downloads-disabled missing dependencies, sandbox-denied localhost listeners, and nested-module fixtures absent from the cached module archive prevent a full-suite green result. Full evidence remains outside the research run at `/Users/zjsng/printing-press/toolchain-fixes/20260927-212939-86ef661f/proofs/local-install-upstream-full-tests.log`. No broad retry was made.

The global installed binary and cached Press source hashes are unchanged. Go resolution did create empty download lock metadata for missing dependencies in the global download cache, despite downloads being disabled; this is recorded in the JSON proof. No cache source was edited and no global install or public write occurred.

Copied source, Go cache, and test fixtures were moved outside the research run so source fixtures are not interpreted as product research. Historical invocation paths in captured test outputs are preserved. The run contains only the patched binary, compact patch, exact local stanza, and comparator/provenance proofs.
