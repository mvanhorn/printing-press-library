// The `tldr` subcommand: find-or-create an archive, extract the article text,
// summarize via LLM, print the summary.
//
// This is the "tl;dr here" workflow Matt asked for during dogfooding. It
// composes the existing read, get, and llmSummarize logic into a single
// one-shot command that runs cleanly from the terminal or from an agent.
//
// Unit 9 of the polish plan.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// newTldrCmd creates the `tldr <url>` subcommand.
func newTldrCmd(flags *rootFlags) *cobra.Command {
	var backend string
	var showRaw bool

	cmd := &cobra.Command{
		Use:   "tldr <url>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Fetch an archived article and summarize it with an LLM",
		Long: "Runs the full paywall-bypass pipeline: find or create an archive.today\n" +
			"snapshot, fetch the body, extract clean text, and summarize via a locally\n" +
			"installed LLM. Output is a 1-line headline plus 3 bullet points.\n\n" +
			"LLM provider selection (first available wins):\n" +
			"  1. `claude` CLI on PATH (recommended)\n" +
			"  2. ANTHROPIC_API_KEY env var (direct api.anthropic.com call)\n" +
			"  3. OPENAI_API_KEY env var (direct api.openai.com call)",
		Example: "  archive-is-pp-cli tldr https://www.nytimes.com/2026/04/10/example-article\n" +
			"  archive-is-pp-cli tldr https://ft.com/content/xyz --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			origURL := args[0]
			if flags.dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "DRY RUN: tldr %s\n", origURL)
				return nil
			}
			if !strings.HasPrefix(origURL, "http://") && !strings.HasPrefix(origURL, "https://") {
				return usageErr(fmt.Errorf("url must start with http:// or https://"))
			}

			summary, m, err := runTldr(cmd, flags, origURL, backend)
			if err != nil {
				return err
			}

			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{
					"original_url": origURL,
					"memento_url":  m.MementoURL,
					"backend":      m.Backend,
					"headline":     summary.Headline,
					"bullets":      summary.Bullets,
					"provider":     summary.Provider,
				})
			}

			// Human-readable output
			if summary.Headline != "" {
				fmt.Fprintln(cmd.OutOrStdout(), summary.Headline)
				fmt.Fprintln(cmd.OutOrStdout())
			}
			for _, b := range summary.Bullets {
				fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", b)
			}
			fmt.Fprintln(cmd.OutOrStdout())
			fmt.Fprintf(cmd.ErrOrStderr(), "  archive: %s\n", m.MementoURL)
			fmt.Fprintf(cmd.ErrOrStderr(), "  via: %s (%s)\n", m.Backend, summary.Provider)
			if showRaw && summary.Raw != "" {
				fmt.Fprintln(cmd.ErrOrStderr(), "\n--- raw LLM response ---")
				fmt.Fprintln(cmd.ErrOrStderr(), summary.Raw)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&backend, "backend", "archive-is,wayback", "Backends to try in order (archive-is, wayback)")
	cmd.Flags().BoolVar(&showRaw, "raw", false, "Print the raw LLM response alongside the parsed summary")
	return cmd
}

// runTldr executes the full pipeline and returns the summary and the memento
// URL used as the source. Also exposed for unit 7 (menu) so the [t] option can
// invoke the same logic without shelling back into the CLI.
func runTldr(cmd *cobra.Command, flags *rootFlags, origURL, backend string) (*Summary, *memento, error) {
	timeout := 30 * time.Second
	if flags.timeout > 0 {
		timeout = flags.timeout
	}

	// 1. Find or create the archive (same logic as read).
	backends := parseBackends(backend)
	var m *memento
	var lookupErr error
	for _, b := range backends {
		switch b {
		case backendArchiveIs:
			m, lookupErr = timegateLookup(origURL, timeout)
		case backendWayback:
			m, lookupErr = waybackLookup(origURL, timeout)
		}
		if m != nil {
			break
		}
	}
	if m == nil {
		if !flags.quiet {
			fmt.Fprintf(cmd.ErrOrStderr(), "No existing archive (%v). Submitting fresh capture...\n", lookupErr)
		}
		var err error
		m, err = runSubmitCapture(cmd, flags, origURL, false)
		if err != nil {
			return nil, nil, classifySubmitError(err)
		}
	}
	m.MementoURL = strings.Replace(m.MementoURL, "http://archive.", "https://archive.", 1)

	// 2. Fetch the body. Fall back to Wayback on archive.is CAPTCHA (same logic as get).
	body, fetchErr := fetchMementoBody(m.MementoURL, 30*time.Second)
	if fetchErr != nil || isCaptchaResponse(body) {
		if !flags.quiet {
			fmt.Fprintln(cmd.ErrOrStderr(), "archive.is returned a CAPTCHA for body fetch, falling back to Wayback Machine...")
		}
		waybackMemento, waybackErr := waybackLookup(origURL, timeout)
		if waybackErr != nil || waybackMemento == nil {
			return nil, nil, apiErr(fmt.Errorf("archive.is CAPTCHA and Wayback lookup failed: %w", waybackErr))
		}
		body, fetchErr = fetchMementoBody(waybackMemento.MementoURL, 30*time.Second)
		if fetchErr != nil {
			return nil, nil, apiErr(fmt.Errorf("fetching wayback memento: %w", fetchErr))
		}
		m = waybackMemento
		if isHardPaywallDomain(origURL) && !flags.quiet && !flags.asJSON {
			fmt.Fprint(cmd.ErrOrStderr(), paywallWarning(origURL))
		}
	}

	// 3. Extract clean text.
	text := extractReadableText(string(body))
	if len(text) < 200 {
		return nil, m, apiErr(fmt.Errorf("extracted body too short (%d chars) to summarize meaningfully", len(text)))
	}

	// 4. Summarize via LLM.
	if !flags.quiet {
		fmt.Fprintln(cmd.ErrOrStderr(), "Summarizing with LLM...")
	}
	summary, err := llmSummarize(text)
	if err != nil {
		return nil, m, apiErr(err)
	}
	return summary, m, nil
}
