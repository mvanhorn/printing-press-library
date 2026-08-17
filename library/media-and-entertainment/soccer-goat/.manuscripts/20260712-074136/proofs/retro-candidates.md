# Retro candidates (generator/framework, not soccer-goat defects)

1. **Generated framework teaching commands lack pp:happy-args.** teach, teach-pattern,
   teach-playbook, and playbook amend require flags but ship without pp:happy-args, so
   `dogfood --live` happy_path/json_fidelity probes fail (exit 2 / empty). The generator
   should emit pp:happy-args for these framework commands.
2. **teach Example uses shell line-continuation backslashes.** dogfood's Example-arg parser
   splits on whitespace and injects a literal `\` (and could inject `&`) as a positional arg,
   corrupting the probe invocation. Generated command Example strings should be single-line
   or dogfood should strip trailing `\`/`&` shell tokens.
3. **dogfood tokenizes pp:happy-args `--flag=value` into `--flag value`**, which breaks
   boolean flags (`--quiet=false` -> `--quiet` true + `false` positional). Either preserve
   the `=value` form for known bool flags, or document that happy-args cannot set bools.
4. **teach --json was gated behind --quiet=false.** A command that advertises the global
   --json flag but only emits JSON when a second flag is set is a footgun; the generator's
   silent-command pattern should honor explicit --json as a machine-output request.
