// Hand-written: company-goat-pp-cli novel commands.
// This file contains the shared target-resolution helper and the
// `resolve` subcommand.

package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/company-goat/internal/resolve"
	"github.com/spf13/cobra"
)

// targetFlags holds the standard set of resolve flags every per-source
// command shares (--domain, --pick, --name).
type targetFlags struct {
	Domain string // explicit domain — short-circuits resolution
	Pick   int    // 1-indexed; pick from the last cached candidate list
}

// Per-command --domain and --pick declarations are inlined in each command
// file rather than wrapped in a shared helper. This is intentional: the
// verify-skill check greps each command's source file for the literal
// `cmd.Flags().StringVar(..., "domain", ...)` declaration and would
// otherwise report false-positive "declared elsewhere but not on <command>"
// errors when the flag is added through indirection.

// resolveTarget runs the multi-source resolver and returns either a
// canonical domain or an error indicating disambiguation needed (exit 2)
// or no candidates (exit 4). Callers print the candidate list themselves
// when ErrAmbiguous is returned.
func resolveTarget(ctx context.Context, args []string, t targetFlags) (string, *resolve.Resolution, error) {
	// --domain wins.
	if strings.TrimSpace(t.Domain) != "" {
		dom := normalizeArg(t.Domain)
		return dom, &resolve.Resolution{AutoResolved: true, Domain: dom, Source: "user-supplied"}, nil
	}
	if len(args) == 0 {
		return "", nil, errors.New("name or --domain required")
	}
	query := strings.Join(args, " ")

	r := resolve.NewResolver()
	res, err := r.Resolve(ctx, query)
	if err != nil {
		return "", nil, fmt.Errorf("resolve: %w", err)
	}
	if res.AutoResolved {
		return res.Domain, res, nil
	}
	if res.NoCandidates {
		return "", res, errNoCandidates
	}
	// Multiple candidates.
	if t.Pick > 0 && t.Pick <= len(res.Candidates) {
		c := res.Candidates[t.Pick-1]
		return c.Domain, &resolve.Resolution{AutoResolved: true, Domain: c.Domain, Source: c.Source + " (--pick)"}, nil
	}
	return "", res, errAmbiguous
}

var (
	errAmbiguous    = errors.New("ambiguous: multiple candidates found")
	errNoCandidates = errors.New("no candidates: try --domain")
)

// emitResolutionDiag writes the resolved domain to stderr so the user
// can verify which target the command actually picked. Skipped when the
// caller passed --domain (no ambiguity).
func emitResolutionDiag(cmd *cobra.Command, res *resolve.Resolution) {
	if res == nil || !res.AutoResolved || res.Source == "user-supplied" {
		return
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "resolved %q → %s (via %s)\n", res.Query, res.Domain, res.Source)
}

// renderCandidates prints a numbered list to stdout and returns exit
// code 2 (caller surfaces).
func renderCandidates(cmd *cobra.Command, flags *rootFlags, res *resolve.Resolution) error {
	w := cmd.OutOrStdout()
	asJSON := flags.asJSON || !isTerminal(w)
	if asJSON {
		type out struct {
			Disambiguation string              `json:"disambiguation"`
			Query          string              `json:"query"`
			Candidates     []resolve.Candidate `json:"candidates"`
			PickCommands   []string            `json:"pick_commands"`
			DomainOverride string              `json:"domain_override_example"`
		}
		picks := make([]string, 0, len(res.Candidates))
		root := cmd.Root().Name()
		for i := range res.Candidates {
			picks = append(picks, fmt.Sprintf("%s %s %q --pick %d", root, cmd.Name(), res.Query, i+1))
		}
		o := out{
			Disambiguation: "needed",
			Query:          res.Query,
			Candidates:     res.Candidates,
			PickCommands:   picks,
			DomainOverride: fmt.Sprintf("%s %s --domain <pick-one-from-candidates>", root, cmd.Name()),
		}
		return flags.printJSON(cmd, o)
	}
	fmt.Fprintf(w, "Multiple matches for %q. Pick one:\n\n", res.Query)
	for i, c := range res.Candidates {
		hint := ""
		if c.Description != "" {
			hint = c.Description
			if len(hint) > 60 {
				hint = hint[:57] + "..."
			}
		}
		yr := ""
		if c.Year != "" {
			yr = " " + c.Year
		}
		fmt.Fprintf(w, "  %d  %-30s  %-50s  [%s%s]\n", i+1, c.Domain, hint, c.Source, yr)
	}
	root := cmd.Root().Name()
	fmt.Fprintf(w, "\nRerun with: %s %s %q --pick N\n", root, cmd.Name(), res.Query)
	fmt.Fprintf(w, "       or:  %s %s --domain <pick-one-from-candidates>\n", root, cmd.Name())
	return nil
}

// renderNoCandidates prints a "not found" message; exit code 4.
func renderNoCandidates(cmd *cobra.Command, flags *rootFlags, query string) error {
	w := cmd.OutOrStdout()
	asJSON := flags.asJSON || !isTerminal(w)
	if asJSON {
		fmt.Fprintf(w, `{"resolution":"no-candidates","query":%q,"hint":"pass --domain explicitly"}`+"\n", query)
		return nil
	}
	fmt.Fprintf(w, "no candidates found for %q\n", query)
	fmt.Fprintf(w, "Try: %s %s --domain <domain.com>\n", cmd.Root().Name(), cmd.Name())
	return nil
}

// runResolveOrExit is the boilerplate every per-source command uses:
//  1. Resolve target.
//  2. On ambiguous → render candidates, exit 2.
//  3. On no-candidates → render hint, exit 4.
//  4. Otherwise return the domain so the command can call its source.
func runResolveOrExit(cmd *cobra.Command, flags *rootFlags, args []string, t targetFlags) (string, error) {
	ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
	defer cancel()

	domain, res, err := resolveTarget(ctx, args, t)
	switch {
	case errors.Is(err, errAmbiguous):
		_ = renderCandidates(cmd, flags, res)
		os.Exit(2)
		return "", nil // unreachable
	case errors.Is(err, errNoCandidates):
		_ = renderNoCandidates(cmd, flags, res.Query)
		os.Exit(4)
		return "", nil
	case err != nil:
		return "", err
	}
	emitResolutionDiag(cmd, res)
	return domain, nil
}

func normalizeArg(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	for _, prefix := range []string{"https://", "http://"} {
		s = strings.TrimPrefix(s, prefix)
	}
	s = strings.TrimPrefix(s, "www.")
	if i := strings.Index(s, "/"); i > 0 {
		s = s[:i]
	}
	return s
}

func newResolveCmd(flags *rootFlags) *cobra.Command {
	var domainFlag string
	var pick int

	cmd := &cobra.Command{
		Use:   "resolve <name-or-domain>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Resolve a company name to a canonical domain. Returns numbered candidates if ambiguous.",
		Long: `Resolve a company name into a canonical domain by querying Wikidata, the YC directory, and DNS probes in parallel.

Exit codes:
  0  exactly one high-confidence match (or input was already a domain)
  2  multiple candidates — rerun with --pick N or --domain
  4  no candidates — pass --domain explicitly`,
		Example: strings.Trim(`
  company-goat-pp-cli resolve stripe
  company-goat-pp-cli resolve stripe --pick 2
  company-goat-pp-cli resolve apollo --json
  company-goat-pp-cli resolve --domain anthropic.com
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(cmd, flags) {
				return nil
			}
			if domainFlag == "" && len(args) == 0 {
				return cmd.Help()
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()

			t := targetFlags{Domain: domainFlag, Pick: pick}
			domain, res, err := resolveTarget(ctx, args, t)
			if errors.Is(err, errAmbiguous) {
				_ = renderCandidates(cmd, flags, res)
				os.Exit(2)
			}
			if errors.Is(err, errNoCandidates) {
				_ = renderNoCandidates(cmd, flags, res.Query)
				os.Exit(4)
			}
			if err != nil {
				return err
			}
			out := map[string]any{
				"domain":        domain,
				"source":        res.Source,
				"query":         res.Query,
				"auto_resolved": res.AutoResolved,
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, out)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "resolved %q → %s (via %s)\n", res.Query, domain, res.Source)
			return nil
		},
	}
	cmd.Flags().StringVar(&domainFlag, "domain", "", "Skip name resolution and use this domain (e.g. stripe.com)")
	cmd.Flags().IntVar(&pick, "pick", 0, "Pick candidate N (1-indexed) from a previous ambiguous resolve")
	return cmd
}
