// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-built novel feature: mechanically scan HTML for leaked secrets and PII.
// ht-ml.app sites are public and have no delete endpoint, so a leaked key in
// published HTML is exposed forever. scan works two ways: as a pre-publish guard
// on a local file/stdin, and as a retroactive audit of already-published sites
// read from the local store (the only way to check what you can no longer
// delete). Read-only. Survives generate --force.
// pp:data-source local

package cli

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// scanPattern is one secret/PII signature. Severity is "high" for credentials
// (always block) or "pii" for personal data (informational unless --strict).
type scanPattern struct {
	name     string
	severity string
	re       *regexp.Regexp
}

// secretPatterns are credential signatures: a hit is almost always a real leak.
var secretPatterns = []scanPattern{
	{"private-key-block", "high", regexp.MustCompile(`-----BEGIN (?:RSA |EC |OPENSSH |DSA |PGP )?PRIVATE KEY-----`)},
	{"aws-access-key-id", "high", regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`)},
	{"google-api-key", "high", regexp.MustCompile(`\bAIza[0-9A-Za-z_\-]{35}\b`)},
	{"github-token", "high", regexp.MustCompile(`\b(?:ghp|gho|ghu|ghs|ghr)_[A-Za-z0-9]{36}\b`)},
	{"github-fine-grained-token", "high", regexp.MustCompile(`\bgithub_pat_[A-Za-z0-9_]{22,}\b`)},
	{"slack-token", "high", regexp.MustCompile(`\bxox[baprs]-[A-Za-z0-9-]{10,}\b`)},
	{"stripe-secret-key", "high", regexp.MustCompile(`\bsk_live_[A-Za-z0-9]{16,}\b`)},
	{"openai-key", "high", regexp.MustCompile(`\bsk-(?:proj-|svcacct-|[A-Za-z0-9])[A-Za-z0-9_\-]{20,}\b`)},
	{"jwt", "high", regexp.MustCompile(`\beyJ[A-Za-z0-9_\-]{10,}\.[A-Za-z0-9_\-]{10,}\.[A-Za-z0-9_\-]{10,}\b`)},
	{"generic-secret-assignment", "high", regexp.MustCompile(`(?i)(?:api[_-]?key|secret|token|passwd|password|access[_-]?key)\s*[:=]\s*['"][A-Za-z0-9_\-\.]{16,}['"]`)},
}

// piiPatterns are personal-data signatures: often intentional on a public page,
// so they are informational unless --strict is set. Scanned only with --pii.
var piiPatterns = []scanPattern{
	{"email", "pii", regexp.MustCompile(`\b[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}\b`)},
	{"us-ssn", "pii", regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`)},
	{"phone-e164", "pii", regexp.MustCompile(`\+\d{10,15}\b`)},
	{"credit-card", "pii", regexp.MustCompile(`\b(?:\d[ -]?){13,16}\b`)},
}

type scanFinding struct {
	Type     string `json:"type"`
	Severity string `json:"severity"`
	Line     int    `json:"line"`
	Preview  string `json:"preview"`
}

type scanResult struct {
	Source   string        `json:"source"`
	Clean    bool          `json:"clean"`
	High     int           `json:"high_severity"`
	PII      int           `json:"pii"`
	Findings []scanFinding `json:"findings"`
}

func newNovelScanCmd(flags *rootFlags) *cobra.Command {
	var htmlFlag string
	var includePII, strict, allSites bool

	cmd := &cobra.Command{
		Use:   "scan [file|-|site_id]",
		Short: "Mechanically scan HTML for leaked secrets and PII before (or after) it becomes a public, permanent URL.",
		Long: trimNL(`
ht-ml.app sites are public and have no delete endpoint, so a credential pasted
into published HTML is exposed permanently. scan checks for credential
signatures (private keys, cloud keys, tokens, JWTs, secret assignments)
entirely locally, with no API call.

Three modes:
  • a local file, '-' for stdin, or --html : pre-publish guard
  • a site_id already in your store         : audit one published site
  • --all-sites                             : audit every site you've published

Matched secrets are redacted in the output (only a short prefix is shown) so the
scan result is itself safe to share. Exit code is non-zero when a credential is
found (use --pii to also flag personal data, --strict to fail on PII too).`),
		Example: trimNL(`
  ht-ml-pp-cli scan ./page.html && ht-ml-pp-cli publish ./page.html
  ht-ml-pp-cli scan e5051f46 --agent
  ht-ml-pp-cli scan --all-sites --pii --agent`),
		// happy-args drives the live-dogfood happy path with a clean literal HTML
		// string (no on-disk fixture, no findings) so the gate runs deterministically.
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--html=<p>clean</p>"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			patterns := append([]scanPattern{}, secretPatterns...)
			if includePII {
				patterns = append(patterns, piiPatterns...)
			}

			// --all-sites and site_id modes read published HTML from the local
			// store; file/stdin/--html mode reads the provided document.
			if allSites {
				return scanAllStoredSites(cmd, flags, patterns, strict)
			}
			if htmlFlag == "" && len(args) >= 1 && args[0] != "-" {
				if _, statErr := os.Stat(args[0]); statErr != nil {
					// Not a readable file — try resolving it as a stored site_id.
					return scanStoredSite(cmd, flags, args[0], patterns, strict)
				}
			}

			html, srcLabel, _, err := readHTMLInput(cmd, args, htmlFlag)
			if err != nil {
				return usageErr(err)
			}
			res := scanHTML(srcLabel, html, patterns, strict)
			return emitScanResult(cmd, flags, res)
		},
	}
	cmd.Flags().StringVar(&htmlFlag, "html", "", "HTML content as a literal string (instead of a file)")
	cmd.Flags().BoolVar(&includePII, "pii", false, "Also scan for personal data (emails, phone numbers, SSNs, card numbers)")
	cmd.Flags().BoolVar(&strict, "strict", false, "Treat PII matches as failures too (non-zero exit)")
	cmd.Flags().BoolVar(&allSites, "all-sites", false, "Audit every published site's stored HTML in the local store")
	return cmd
}

// scanHTML applies every pattern to one document and returns its result.
func scanHTML(source, html string, patterns []scanPattern, strict bool) scanResult {
	lines := strings.Split(html, "\n")
	result := scanResult{Source: source}
	for i, line := range lines {
		for _, p := range patterns {
			for _, m := range p.re.FindAllString(line, -1) {
				if p.name == "credit-card" && !looksLikeCard(m) {
					continue
				}
				result.Findings = append(result.Findings, scanFinding{
					Type:     p.name,
					Severity: p.severity,
					Line:     i + 1,
					Preview:  redactSecret(m),
				})
				if p.severity == "high" {
					result.High++
				} else {
					result.PII++
				}
			}
		}
	}
	sort.SliceStable(result.Findings, func(i, j int) bool {
		return result.Findings[i].Line < result.Findings[j].Line
	})
	result.Clean = result.High == 0 && (!strict || result.PII == 0)
	return result
}

// scanStoredSite audits a single already-published site's latest stored HTML.
func scanStoredSite(cmd *cobra.Command, flags *rootFlags, siteID string, patterns []scanPattern, strict bool) error {
	ctx, cancel := boundCtx(cmd.Context(), flags)
	defer cancel()
	db, err := htmlxOpenStore(ctx, flags)
	if err != nil {
		return err
	}
	defer db.Close()
	site, err := db.GetSite(siteID)
	if err != nil {
		return err
	}
	if site == nil {
		return usageErr(fmt.Errorf("%q is neither a readable file nor a site_id in the local store", siteID))
	}
	versions, err := db.ListVersions(siteID)
	if err != nil {
		return err
	}
	if len(versions) == 0 {
		return notFoundErr(fmt.Errorf("no stored HTML for site %q to scan", siteID))
	}
	res := scanHTML(siteID, versions[0].HTML, patterns, strict)
	return emitScanResult(cmd, flags, res)
}

// scanAllStoredSites audits every published site's latest stored HTML.
func scanAllStoredSites(cmd *cobra.Command, flags *rootFlags, patterns []scanPattern, strict bool) error {
	ctx, cancel := boundCtx(cmd.Context(), flags)
	defer cancel()
	db, err := htmlxOpenStore(ctx, flags)
	if err != nil {
		return err
	}
	defer db.Close()
	sites, err := db.ListSites()
	if err != nil {
		return err
	}
	results := make([]scanResult, 0, len(sites))
	anyUnclean := false
	for _, s := range sites {
		versions, verr := db.ListVersions(s.SiteID)
		if verr != nil || len(versions) == 0 {
			continue
		}
		res := scanHTML(s.SiteID, versions[0].HTML, patterns, strict)
		if !res.Clean {
			anyUnclean = true
		}
		results = append(results, res)
	}

	if wantsHumanTable(cmd.OutOrStdout(), flags) {
		w := cmd.OutOrStdout()
		if len(results) == 0 {
			fmt.Fprintln(w, "no published sites with stored HTML to scan")
			return nil
		}
		tw := newTabWriter(w)
		fmt.Fprintln(tw, bold("SITE_ID")+"\t"+bold("SECRETS")+"\t"+bold("PII"))
		for _, r := range results {
			secrets := green("0")
			if r.High > 0 {
				secrets = red(fmt.Sprintf("%d", r.High))
			}
			fmt.Fprintf(tw, "%s\t%s\t%d\n", r.Source, secrets, r.PII)
		}
		if err := tw.Flush(); err != nil {
			return err
		}
	} else if err := printJSONFiltered(cmd.OutOrStdout(), results, flags); err != nil {
		return err
	}

	if anyUnclean {
		return &cliError{code: 8, err: fmt.Errorf("at least one published site has leaked secrets; they cannot be deleted, only overwritten via 'update'")}
	}
	return nil
}

// emitScanResult prints a single scanResult and returns the exit-code error when
// it is not clean.
func emitScanResult(cmd *cobra.Command, flags *rootFlags, result scanResult) error {
	if wantsHumanTable(cmd.OutOrStdout(), flags) {
		w := cmd.OutOrStdout()
		if len(result.Findings) == 0 {
			fmt.Fprintf(w, "%s no secrets found in %s\n", green("clean:"), result.Source)
		} else {
			tw := newTabWriter(w)
			fmt.Fprintln(tw, bold("LINE")+"\t"+bold("SEVERITY")+"\t"+bold("TYPE")+"\t"+bold("PREVIEW"))
			for _, f := range result.Findings {
				sev := f.Severity
				if f.Severity == "high" {
					sev = red(f.Severity)
				}
				fmt.Fprintf(tw, "%d\t%s\t%s\t%s\n", f.Line, sev, f.Type, f.Preview)
			}
			_ = tw.Flush()
			fmt.Fprintf(w, "%s %d secret(s), %d PII match(es) in %s\n", red("found:"), result.High, result.PII, result.Source)
		}
	} else if err := printJSONFiltered(cmd.OutOrStdout(), result, flags); err != nil {
		return err
	}

	if !result.Clean {
		return &cliError{code: 8, err: fmt.Errorf("scan found %d high-severity secret(s) and %d PII match(es); do not publish until resolved", result.High, result.PII)}
	}
	return nil
}

// redactSecret returns a safe-to-display version of a matched secret: only a
// short leading prefix (which is usually the non-secret type marker, e.g. AKIA,
// ghp_, eyJ) followed by a fixed mask. The secret body is never emitted.
func redactSecret(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "-----BEGIN") {
		return "-----BEGIN PRIVATE KEY----- (block)"
	}
	prefix := 4
	if len(s) < prefix {
		return strings.Repeat("*", len(s))
	}
	return s[:prefix] + "****"
}

// looksLikeCard applies the Luhn check so the broad credit-card regex does not
// flag arbitrary digit runs (timestamps, ids).
func looksLikeCard(s string) bool {
	digits := make([]int, 0, 16)
	for _, r := range s {
		if r >= '0' && r <= '9' {
			digits = append(digits, int(r-'0'))
		}
	}
	if len(digits) < 13 || len(digits) > 16 {
		return false
	}
	sum := 0
	double := false
	for i := len(digits) - 1; i >= 0; i-- {
		d := digits[i]
		if double {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
		double = !double
	}
	return sum%10 == 0
}
