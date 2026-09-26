// pp:data-source local

package cli

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/thunderbird/internal/tbprofile"
	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		if parent := tbFindChild(root, "messages"); parent != nil {
			addNovelCommandIfAbsent(parent, newTBMessagesAuthCmd(flags))
			addNovelCommandIfAbsent(parent, newTBMessagesExportCmd(flags))
		}
	})
}

type tbAuthVerdict struct {
	Method  string `json:"method"`
	Result  string `json:"result"`
	Details string `json:"details"`
	Source  string `json:"source"`
}

type tbAuthReport struct {
	ID                    string          `json:"id"`
	Subject               string          `json:"subject"`
	FromAddr              string          `json:"from_addr"`
	SPF                   string          `json:"spf"`
	DKIM                  string          `json:"dkim"`
	DMARC                 string          `json:"dmarc"`
	Verdicts              []tbAuthVerdict `json:"verdicts"`
	AuthenticationResults []string        `json:"authentication_results"`
	ReceivedSPF           []string        `json:"received_spf"`
	Source                string          `json:"source"`
}

var tbCommentRE = regexp.MustCompile(`\([^()]*\)`)

// tbParseAuthResults splits Authentication-Results values (RFC 8601) into
// method=result verdicts; the authserv-id before the first ";" is skipped.
func tbParseAuthResults(values []string) []tbAuthVerdict {
	out := make([]tbAuthVerdict, 0)
	for _, v := range values {
		v = tbCommentRE.ReplaceAllString(strings.Join(strings.Fields(v), " "), "")
		parts := strings.Split(v, ";")
		for _, p := range parts[1:] {
			p = strings.TrimSpace(p)
			method, rest, ok := strings.Cut(p, "=")
			if !ok || strings.ContainsAny(method, " \t") {
				continue
			}
			result, details, _ := strings.Cut(strings.TrimSpace(rest), " ")
			out = append(out, tbAuthVerdict{Method: strings.ToLower(method), Result: strings.ToLower(result), Details: strings.TrimSpace(details), Source: "Authentication-Results"})
		}
	}
	return out
}

// tbAuthSummary picks one verdict per method: pass wins for dkim (any valid
// signature), otherwise the first reported result.
func tbAuthSummary(rep *tbAuthReport, receivedSPF []string) {
	for _, v := range rep.Verdicts {
		switch v.Method {
		case "spf":
			if rep.SPF == "" {
				rep.SPF = v.Result
			}
		case "dkim":
			if rep.DKIM == "" || v.Result == "pass" {
				rep.DKIM = v.Result
			}
		case "dmarc":
			if rep.DMARC == "" {
				rep.DMARC = v.Result
			}
		}
	}
	for _, r := range receivedSPF {
		result, details, _ := strings.Cut(strings.TrimSpace(r), " ")
		v := tbAuthVerdict{Method: "spf", Result: strings.ToLower(result), Details: strings.TrimSpace(details), Source: "Received-SPF"}
		rep.Verdicts = append(rep.Verdicts, v)
		if rep.SPF == "" {
			rep.SPF = v.Result
		}
	}
}

func newTBMessagesAuthCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth <id>",
		Short: "Show SPF, DKIM and DMARC verdicts of a message",
		Long: `Parse the Authentication-Results and Received-SPF headers of a message into
spf, dkim and dmarc verdicts (pass, fail, softfail, none, ...) plus the raw
header values. An empty verdict means the receiving server did not report it.`,
		Example: strings.Trim(`
  thunderbird-pp-cli messages auth 3f9a1c2b7d4e
  thunderbird-pp-cli messages auth 3f9a1c2b7d4e --json --select spf,dkim,dmarc`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local", "pp:typed-exit-codes": "0,2,3", "pp:happy-args": "id=0123456789ab"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "messages auth")
			}
			if len(args) != 1 {
				return usageErr(fmt.Errorf("expected exactly one message id\nUsage: %s <id>", cmd.CommandPath()))
			}
			db, err := tbStoreFor(cmd, flags, "messages")
			if err != nil || db == nil {
				return err
			}
			d, err := tbGetMessage(db, args[0])
			_ = db.Close()
			if err != nil {
				return err
			}
			rep := tbAuthReport{ID: d.ID, Subject: d.Subject, FromAddr: d.FromAddr, Source: "mbox"}
			var received []string
			if raw, rawErr := tbReadRaw(d); rawErr == nil {
				h, _ := tbprofile.SplitMessage(raw)
				rep.AuthenticationResults = h.Values("Authentication-Results")
				received = h.Values("Received-Spf")
			} else {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %v; using the stored Authentication-Results\n", rawErr)
				rep.Source = "store"
				if d.AuthResults != "" {
					rep.AuthenticationResults = strings.Split(d.AuthResults, "\n")
				}
			}
			if rep.AuthenticationResults == nil {
				rep.AuthenticationResults = []string{}
			}
			rep.ReceivedSPF = append([]string{}, received...)
			rep.Verdicts = tbParseAuthResults(rep.AuthenticationResults)
			tbAuthSummary(&rep, received)
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), rep, flags)
			}
			w := tbHumanOut(cmd)
			show := func(s string) string {
				if s == "" {
					return "(not reported)"
				}
				return s
			}
			fmt.Fprintf(w, "SPF:   %s\nDKIM:  %s\nDMARC: %s\n", show(rep.SPF), show(rep.DKIM), show(rep.DMARC))
			for _, v := range rep.Verdicts {
				fmt.Fprintf(w, "  %s=%s %s [%s]\n", v.Method, v.Result, v.Details, v.Source)
			}
			return nil
		},
	}
	return cmd
}
