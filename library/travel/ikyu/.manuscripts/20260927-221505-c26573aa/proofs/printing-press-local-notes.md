# Printing Press integration notes

Version 4.32.5, captured binary retained. Shared tool configuration was not changed.

- Source discovery requires literal Cobra command declarations; runtime-working shared constructors were replaced with equivalent literals. The five approved feature paths now resolve mechanically.
- WorkflowStep.Args is declared but executeStep ignores it. The live workflow therefore keeps executable flags in command strings. checkWorkflowCompleteness scans every token, including variable placeholders and literal dates, against root/subcommand help; its unmapped-step warnings are false positives. The binary-owned live workflow gate passes all three actual steps. No help text was padded and no gate result was edited.
- The reimplementation heuristic does not follow the shared fetchOfferRows helper into the live Ikyu client; compare/dates source calls are verified by independent live results and bounded fan-out tests.
- Multi-ID Use signatures must mark each positional individually; combined square brackets are misread as zero positionals by verify-skill. Required angle-bracket arguments preserve the actual three-ID command contract.
- The framework compact mode trims fields but pretty-prints JSON and can decode through float64. Owned stay output uses typed bounds and a final JSON compaction pass; exact integer-yen assertions cover the path.
- Generic generated store/client static-analysis findings were reviewed separately in gosec-template-triage.json. They were not blanket-suppressed or patched in generator-reserved code.
