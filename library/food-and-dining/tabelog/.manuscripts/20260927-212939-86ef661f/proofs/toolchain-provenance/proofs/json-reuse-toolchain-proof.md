# Exact JSON-run reuse repair

The live-dogfood runner validates its captured successful happy run when the JSON invocation would execute exactly the same arguments. Different args and failed happy runs retain separate execution. All existing JSON and error validation remains active.

Before: counter-based explicit --json and --json=true mutations execute twice. After: both execute once; different-mode probes execute twice; failed runs remain failures; invalid captured JSON remains a fidelity failure. Focused tests and prior overlay regressions pass. No user state reload, default removal-error change, source-check skip, product edit, receipt or public write is part of this repair.

Final binary: `/Users/zjsng/printing-press/.runstate/zjsng-a9b2d4f4/runs/20260927-212939-86ef661f/toolchain/bin/cli-printing-press`

SHA-256: `551d228e887fb3f0774efc8e0bb1ee6beed6a3837847c652b3254be8cb787bfd`

Separate patch: `/Users/zjsng/printing-press/.runstate/zjsng-a9b2d4f4/runs/20260927-212939-86ef661f/toolchain/live-dogfood-json-reuse.patch`

This binary includes all three run-local upstream repairs. Source/cache/baseline artifacts remain outside research RUN.
