# Scorecard fixture-overlay repair

The scorecard now applies the existing annotation overlay before execution, using actual command help to parse flags/positionals. Both runners share explicit Example home precedence and checkout-relative annotation-home resolution. Ordinary examples and mutation guards are unchanged.

The same executable regression fails upstream and passes the patch; focused home/argument tests and existing happy-args regressions pass. Baseline and after logs are adjacent. No product gate, receipt, public write or removal-behavior change is part of this repair.

Binary: `/Users/zjsng/printing-press/.runstate/zjsng-a9b2d4f4/runs/20260927-212939-86ef661f/toolchain/bin/cli-printing-press`

SHA-256: `6f2964ba9184c9bfc1252681dff0d7d03d6846cfc08bb1f4985b69fbd9d26d28`

Patch: `/Users/zjsng/printing-press/.runstate/zjsng-a9b2d4f4/runs/20260927-212939-86ef661f/toolchain/scorecard-happy-args.patch`

Source, baseline and caches remain outside research RUN. Global module-source directories are read through links; all new module metadata and build cache writes use the local toolchain directories.
