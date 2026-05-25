// Copyright 2026 vinny-pasceri. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// PATCH(amend-2026-05-18: types-search-discoverability): wraps the
// generated 'types search' command with three discoverability fixes
// requested in pp-numista/AGENTS.md Priority 1 and Priority 3:
//
//   * Append a Tip block to --help that names the recommended filter combo
//     (--q + --issuer + --year/--date) and gives a working example. (P3.1)
//   * Append a slug-convention note to the --issuer usage string and
//     point to 'issuers find <name>' for discovery. (P1.3)
//   * On HTTP 400 with "Invalid value for parameter 'issuer'", intercept
//     the error returned by the generated RunE, look up the user-supplied
//     --issuer value in the LOCAL cache only (never spend a quota call on
//     the error path), and append "Did you mean: <slug>, ..." with the
//     top 3 fuzzy matches. Cache empty → instruct the agent to run
//     'numista-pp-cli issuers' to warm it. (P1.1)
//   * After a successful run with a q-only filter on a TTY in non-machine
//     mode, print a stderr hint nudging toward narrowing flags. (P3.2)
//
// All edits in this file are hand-written. The generated types_search.go
// stays untouched so a future regeneration does not lose these fixes — the
// attach step walks the cobra tree at root-cmd construction time and
// decorates the live command.

// attachTypesSearchAmendments wires the help-text, error-suggestion, and
// post-run hint onto the generated 'types search' command. Called from
// root.go AFTER newTypesCmd has been added to rootCmd so the search
// subcommand is already attached.
func attachTypesSearchAmendments(rootCmd *cobra.Command, flags *rootFlags) {
	searchCmd := findSubcommand(rootCmd, "types", "search")
	if searchCmd == nil {
		return
	}

	// P3.1: append the Tip block to Long. We preserve the existing Long
	// (currently empty in the generated source — cobra falls back to Short)
	// and append a paragraph; the help renderer will word-wrap as needed.
	tip := "\n\nTip: --q alone is often noisy for common nouns ('morgan', 'eagle', 'dollar')\n" +
		"and returns results across many issuers and eras. Narrow with --issuer (slug) and\n" +
		"--year/--date when you know the era. Example:\n" +
		"  numista-pp-cli types search --q morgan --issuer united-states --year 1878-1921\n" +
		"\n" +
		"Issuer codes are hyphenated slugs (e.g. 'united-states', 'south-africa').\n" +
		"Run 'numista-pp-cli issuers find <name>' to look one up — local-only, no quota cost."
	if searchCmd.Long == "" {
		searchCmd.Long = searchCmd.Short + tip
	} else {
		searchCmd.Long += tip
	}

	// P1.3: append slug-convention reminder to the --issuer flag usage.
	// The cobra help renderer prints the usage string verbatim under the
	// flag name, so a single appended sentence is the right shape here.
	if issuerFlag := searchCmd.Flags().Lookup("issuer"); issuerFlag != nil {
		const slugNote = " Hyphenated slug (e.g. 'united-states'). Run 'numista-pp-cli issuers find <name>' to look one up."
		if !strings.Contains(issuerFlag.Usage, "issuers find") {
			issuerFlag.Usage += slugNote
		}
	}

	// P1.1 + P3.2: wrap RunE to fold in error-suggestion + post-success
	// narrowing hint. Capture the original RunE so we keep generated
	// behavior unchanged on the happy path.
	origRunE := searchCmd.RunE
	if origRunE == nil {
		return
	}
	searchCmd.RunE = func(cmd *cobra.Command, args []string) error {
		// Capture the user-supplied --issuer BEFORE running so we can
		// surface it in the suggestion even when the API echoes a
		// normalized form. cobra.Flag.Value is the live binding; reading
		// it before RunE is safe because cobra has already parsed flags.
		userIssuer, _ := cmd.Flags().GetString("issuer")

		runErr := origRunE(cmd, args)
		if runErr != nil {
			return maybeAppendIssuerSuggestion(runErr, userIssuer)
		}
		emitNarrowingHintIfApplicable(cmd, flags)
		return nil
	}
}

// findSubcommand walks parent/child by name. Returns nil if any step is
// missing — caller is expected to no-op rather than fatally fail, so a
// regeneration that legitimately removes a path doesn't crash startup.
func findSubcommand(rootCmd *cobra.Command, parent, child string) *cobra.Command {
	for _, c := range rootCmd.Commands() {
		if c.Name() != parent {
			continue
		}
		for _, cc := range c.Commands() {
			if cc.Name() == child {
				return cc
			}
		}
	}
	return nil
}

// maybeAppendIssuerSuggestion is the error-rewriter for P1.1. The
// classifyAPIError helper in helpers.go wraps the raw API error before
// it reaches us; we test against the wrapped error's message rather
// than the underlying HTTP code so we catch the case regardless of
// classification path. We only suggest when the message clearly mentions
// the 'issuer' parameter — other 400s (e.g. invalid --date) fall through
// untouched.
//
// CRITICAL: this path NEVER hits the API. Suggestions come from the
// local issuers cache only; an empty cache yields the warm-the-cache
// hint, not a silent fetch.
func maybeAppendIssuerSuggestion(runErr error, userIssuer string) error {
	if runErr == nil {
		return nil
	}
	msg := runErr.Error()
	if !strings.Contains(msg, "HTTP 400") {
		return runErr
	}
	if !strings.Contains(strings.ToLower(msg), "issuer") {
		return runErr
	}
	if userIssuer == "" {
		// Nothing to suggest against.
		return runErr
	}
	matches, lookupErr := suggestIssuerSlugs(userIssuer)
	if errors.Is(lookupErr, errIssuersCacheEmpty) {
		hint := fmt.Sprintf("\nhint: the issuer code %q looks invalid (Numista uses hyphenated slugs like 'united-states').\n"+
			"      The local issuers cache is empty, so I can't suggest the right slug.\n"+
			"      Run 'numista-pp-cli issuers' once to populate the cache, then re-run this command.", userIssuer)
		return fmt.Errorf("%w%s", runErr, hint)
	}
	if lookupErr != nil {
		// Read errors against the local cache are non-fatal — the
		// original API error is still the right thing to surface.
		return runErr
	}
	if len(matches) == 0 {
		hint := fmt.Sprintf("\nhint: the issuer code %q didn't match any slug in the local cache.\n"+
			"      Run 'numista-pp-cli issuers find <name>' to browse — slugs are hyphenated\n"+
			"      (e.g. 'united-states', 'south-africa').", userIssuer)
		return fmt.Errorf("%w%s", runErr, hint)
	}
	slugs := make([]string, len(matches))
	for i, m := range matches {
		slugs[i] = m.Slug
	}
	hint := fmt.Sprintf("\nhint: %q is not a valid issuer slug. Did you mean: %s?\n"+
		"      Run 'numista-pp-cli issuers find <name>' to browse — slugs are hyphenated.",
		userIssuer, strings.Join(slugs, ", "))
	return fmt.Errorf("%w%s", runErr, hint)
}

// emitNarrowingHintIfApplicable prints the P3.2 stderr nudge after a
// successful 'types search' run. Gating rules — ALL must hold:
//
//   - --q was set AND none of --issuer/--year/--date/--catalogue were set
//     (q-only is the failure mode AGENTS.md called out).
//   - stdout is a terminal (so we don't pollute machine consumers).
//   - no machine-format flag was set: --json, --csv, --compact, --quiet,
//     --plain, --select, --agent. wantsHumanTable already encodes these
//     checks; reuse it so we never drift from the rest of the CLI.
//
// We deliberately do NOT count-gate the hint. Counting results requires
// intercepting the output buffer, which doubles the complexity for a
// marginal UX gain — the hint is true for any q-only search regardless
// of count, and a single line on stderr is cheap when the user IS at
// the terminal.
func emitNarrowingHintIfApplicable(cmd *cobra.Command, flags *rootFlags) {
	if flags == nil {
		return
	}
	// agent shorthand expands to --json --compact --no-input --no-color --yes;
	// wantsHumanTable handles asJSON/compact but flags.agent may have been
	// set before PersistentPreRunE folded those into asJSON. Belt and braces:
	// gate on agent too so a future refactor of --agent's expansion doesn't
	// silently re-enable the hint.
	if flags.agent {
		return
	}
	if !wantsHumanTable(cmd.OutOrStdout(), flags) {
		return
	}
	q, _ := cmd.Flags().GetString("q")
	if q == "" {
		return
	}
	issuer, _ := cmd.Flags().GetString("issuer")
	year, _ := cmd.Flags().GetString("year")
	date, _ := cmd.Flags().GetString("date")
	catalogue, _ := cmd.Flags().GetInt("catalogue")
	if issuer != "" || year != "" || date != "" || catalogue != 0 {
		return
	}
	// Use cmd.ErrOrStderr() not os.Stderr — every other diagnostic in this
	// CLI goes through the command's error writer (see issuers_amend.go's
	// renderIssuersFind for the documented rationale), and Cobra-based
	// tests can only capture output that flows through it.
	w := cmd.ErrOrStderr()
	fmt.Fprintln(w,
		"# tip: --q alone often returns results across many issuers and eras.")
	fmt.Fprintln(w,
		"#      Narrow with --issuer (e.g. united-states) or --year (e.g. 1878-1921).")
	fmt.Fprintln(w,
		"#      Run 'numista-pp-cli issuers find <name>' to look up an issuer slug.")
}
