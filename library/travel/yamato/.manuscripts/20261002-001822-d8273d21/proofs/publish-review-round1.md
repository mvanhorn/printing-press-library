# First publication review repairs

Greptile identified three generated-framework issues in PR #2221. All were fixed in the Yamato package and recorded in individual regeneration patch records.

1. [MCP server file input](https://github.com/mvanhorn/printing-press-library/pull/2221#discussion_r4162267976): notes-file, playbook-file and playbook-notes-file are excluded from the MCP schema and allowed argument names. A regression test demonstrates that each is rejected before any companion CLI executes; inline notes remain available.
2. [Atomic file delivery](https://github.com/mvanhorn/printing-press-library/pull/2221#discussion_r4162267980): delivery uses unique exclusively created 0600 temporary files in the destination directory, closes them before rename, and cleans them up on error. Tests reproduced the prior symlink overwrite and concurrent rename failures, then proved the unrelated symlink target remains unchanged and 24 concurrent deliveries leave one complete file.
3. [HTTP connection limits](https://github.com/mvanhorn/printing-press-library/pull/2221#discussion_r4162267984): the optional authenticated MCP HTTP server now has a 10-second header-read limit and 2-minute idle timeout. No response-write deadline interrupts long-lived MCP responses. Existing token/TLS/loopback tests and the complete Go suite pass.

Fresh full live dogfood and publication validation pass after these repairs. All six luggage-planning commands retain their behavior and source evidence constraints.
