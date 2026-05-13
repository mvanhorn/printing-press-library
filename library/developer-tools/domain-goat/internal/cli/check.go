// Command: check — availability check (RDAP → WHOIS → DNS fallback)
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/domain-goat/internal/gen"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/domain-goat/internal/scoring"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/domain-goat/internal/source/dnssrc"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/domain-goat/internal/source/rdap"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/domain-goat/internal/source/whoissrc"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/domain-goat/internal/store"
)

// AvailabilityResult is the canonical shape returned by `check` and used by
// other commands (compare, drops, watch run).
type AvailabilityResult struct {
	FQDN        string         `json:"fqdn"`
	Available   bool           `json:"available"`
	Source      string         `json:"source"` // rdap | whois | dns | cache
	Status      string         `json:"status,omitempty"`
	StatusList  []string       `json:"status_list,omitempty"`
	CreatedAt   string         `json:"created_at,omitempty"`
	ExpiresAt   string         `json:"expires_at,omitempty"`
	Registrar   string         `json:"registrar,omitempty"`
	NameServers []string       `json:"name_servers,omitempty"`
	Premium     bool           `json:"premium,omitempty"`
	Price       *PriceInfo     `json:"price,omitempty"`
	Score       *scoring.Score `json:"score,omitempty"`
	Error       string         `json:"error,omitempty"`
	CheckedAt   time.Time      `json:"checked_at"`
}

// PriceInfo holds the Porkbun pricing snapshot for the domain's TLD.
type PriceInfo struct {
	Registrar    string  `json:"registrar"`
	Registration float64 `json:"registration"`
	Renewal      float64 `json:"renewal"`
	Transfer     float64 `json:"transfer"`
}

func newCheckCmd(flags *rootFlags) *cobra.Command {
	var tlds string
	var fileInput string
	var preferSource string
	var includeScore bool
	var includePrice bool
	var parallel int

	cmd := &cobra.Command{
		Use:   "check [domain...]",
		Short: "Check domain availability (RDAP → WHOIS → DNS fallback).",
		Long: `Check whether one or more domains are available. Uses RDAP first
(HTTP/JSON, fast, accurate via 404), falls back to WHOIS port 43, then DNS
heuristics. Add --tlds to expand each name across multiple TLDs. Add --file
to read names from a file or stdin (one per line).`,
		Example: `  domain-goat-pp-cli check example.com
  domain-goat-pp-cli check kindred --tlds com,io,ai,studio
  domain-goat-pp-cli check --file names.txt --tlds com,io --json
  domain-goat-pp-cli check example.com --include-price --include-score --agent`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Read additional names from file
			if fileInput != "" {
				more, err := readNamesFile(fileInput)
				if err != nil {
					return usageErr(err)
				}
				args = append(args, more...)
			}
			if len(args) == 0 {
				return cmd.Help()
			}

			// Expand by tld matrix if --tlds set; otherwise treat each arg as a full FQDN
			tldsList := joinTLDs(tlds)
			var targets []string
			if len(tldsList) > 0 {
				// each arg without a dot is treated as a label; with a dot is treated literally
				labels := []string{}
				literal := []string{}
				for _, a := range args {
					a = strings.ToLower(strings.TrimSpace(a))
					if strings.Contains(a, ".") {
						literal = append(literal, a)
					} else {
						labels = append(labels, a)
					}
				}
				targets = append(targets, literal...)
				targets = append(targets, gen.Pair(labels, tldsList)...)
			} else {
				targets = append(targets, args...)
			}

			fqdns, err := normalizeAll(targets)
			if err != nil {
				return usageErr(err)
			}

			if dryRunOK(flags) {
				return emitJSON(cmd, flags, map[string]any{
					"dry_run": true,
					"targets": fqdns,
				})
			}

			s, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()

			ctx := cmd.Context()
			results := checkParallel(ctx, s, fqdns, preferSource, includeScore, includePrice, parallel)

			if wantJSON(cmd, flags) {
				return emitJSON(cmd, flags, results)
			}
			return renderCheckTable(cmd, results)
		},
	}
	cmd.Flags().StringVar(&tlds, "tlds", "", "Comma-separated TLD list; expand each name across these")
	cmd.Flags().StringVar(&fileInput, "file", "", "Read additional names from file (or '-' for stdin)")
	cmd.Flags().StringVar(&preferSource, "source", "auto", "Preferred source: auto|rdap|whois|dns")
	cmd.Flags().BoolVar(&includeScore, "include-score", false, "Compute brandability score per result")
	cmd.Flags().BoolVar(&includePrice, "include-price", false, "Attach Porkbun pricing per result (requires `pricing sync`)")
	cmd.Flags().IntVar(&parallel, "parallel", 8, "Max concurrent lookups")
	return cmd
}

func checkParallel(ctx context.Context, s *store.Store, fqdns []string, prefer string, addScore, addPrice bool, parallel int) []AvailabilityResult {
	if parallel <= 0 {
		parallel = 8
	}
	sem := make(chan struct{}, parallel)
	results := make([]AvailabilityResult, len(fqdns))
	var wg sync.WaitGroup
	for i, f := range fqdns {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, fqdn string) {
			defer wg.Done()
			defer func() { <-sem }()
			results[idx] = checkOne(ctx, s, fqdn, prefer, addScore, addPrice)
		}(i, f)
	}
	wg.Wait()
	return results
}

// checkOne runs the availability cascade for one domain.
func checkOne(ctx context.Context, s *store.Store, fqdn, prefer string, addScore, addPrice bool) AvailabilityResult {
	r := AvailabilityResult{FQDN: fqdn, CheckedAt: time.Now().UTC()}

	tryRDAP := prefer == "auto" || prefer == "rdap"
	tryWhois := prefer == "auto" || prefer == "whois"
	tryDNS := prefer == "auto" || prefer == "dns"

	if tryRDAP {
		ctx2, cancel := context.WithTimeout(ctx, 15*time.Second)
		res, err := rdap.Lookup(ctx2, fqdn)
		cancel()
		if res != nil {
			r.Source = "rdap"
			r.Available = res.Available
			r.StatusList = res.Status
			r.Status = res.StatusText
			r.CreatedAt = res.CreatedAt()
			r.ExpiresAt = res.ExpiresAt()
			// persist for transcendence commands
			_ = s.SaveRDAPRecord(ctx, fqdn, string(res.Raw), r.Status, res.EventsJSON())
		}
		if err == nil && res != nil {
			finish(s, &r, addScore, addPrice, ctx)
			return r
		}
		if prefer == "rdap" {
			r.Source = "rdap"
			if err != nil {
				r.Error = err.Error()
			}
			finish(s, &r, addScore, addPrice, ctx)
			return r
		}
	}

	if tryWhois {
		ctx2, cancel := context.WithTimeout(ctx, 18*time.Second)
		res, err := whoissrc.Lookup(ctx2, fqdn)
		cancel()
		if res != nil {
			r.Source = "whois"
			r.Available = res.Available
			r.StatusList = res.Status
			r.Status = strings.Join(res.Status, ",")
			r.Registrar = res.Registrar
			r.CreatedAt = res.CreatedAt
			r.ExpiresAt = res.ExpiresAt
			r.NameServers = res.NameServers
			_ = s.SaveWhoisRecord(ctx, fqdn, res.Raw, res.ParsedJSONString(), "port-43")
		}
		if err == nil && res != nil {
			finish(s, &r, addScore, addPrice, ctx)
			return r
		}
		if prefer == "whois" {
			if err != nil {
				r.Error = err.Error()
			}
			finish(s, &r, addScore, addPrice, ctx)
			return r
		}
	}

	if tryDNS {
		ctx2, cancel := context.WithTimeout(ctx, 6*time.Second)
		res, _ := dnssrc.Probe(ctx2, fqdn)
		cancel()
		if res != nil {
			r.Source = "dns"
			r.Available = res.Available
			r.Status = "dns-heuristic"
			r.NameServers = res.NS
		}
	}

	finish(s, &r, addScore, addPrice, ctx)
	return r
}

func finish(s *store.Store, r *AvailabilityResult, addScore, addPrice bool, ctx context.Context) {
	if addScore {
		score := scoring.Compute(r.FQDN)
		r.Score = &score
	}
	if addPrice {
		p, _ := s.PricingForFQDN(ctx, r.FQDN)
		if p != nil {
			r.Price = &PriceInfo{
				Registrar:    p.Registrar,
				Registration: p.Registration,
				Renewal:      p.Renewal,
				Transfer:     p.Transfer,
			}
		}
	}
	// Persist a domain row for compare / shortlist / etc.
	parts := strings.SplitN(r.FQDN, ".", 2)
	label := r.FQDN
	tld := ""
	if len(parts) == 2 {
		label = parts[0]
		tld = parts[1]
	}
	scoreVal := 0
	scoreJSON := ""
	if r.Score != nil {
		scoreVal = r.Score.Total
		scoreJSON = string(rawJSON(r.Score))
	}
	status := r.Status
	if r.Available && status == "" {
		status = "available"
	}
	_ = s.UpsertDomain(ctx, store.DomainRow{
		FQDN: r.FQDN, ASCII: r.FQDN, Label: label, TLD: tld, Length: len(label),
		Score: scoreVal, ScoreJSON: scoreJSON,
		Status: status, Source: r.Source, Premium: r.Premium,
		CreatedAt: r.CreatedAt, ExpiresAt: r.ExpiresAt,
		LastCheckedAt: r.CheckedAt.Format(time.RFC3339),
	})
}

func renderCheckTable(cmd *cobra.Command, results []AvailabilityResult) error {
	tw := newTabWriter(cmd.OutOrStdout())
	fmt.Fprintln(tw, "DOMAIN\tAVAILABLE\tSOURCE\tSTATUS\tEXPIRES\tPRICE")
	for _, r := range results {
		avail := "no"
		if r.Available {
			avail = "yes"
		}
		price := ""
		if r.Price != nil {
			price = fmt.Sprintf("$%.2f", r.Price.Registration)
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
			r.FQDN, avail, r.Source, truncate(r.Status, 30), r.ExpiresAt, price)
	}
	return tw.Flush()
}

// PATCH(check-stdin-buffered-read): readNamesFile reads all input into a single []byte before JSON vs plain-text branching — opening stdin twice silently truncated piped plain-text because the json.Decoder buffered ~512 bytes internally.
func readNamesFile(path string) ([]string, error) {
	// Read once into memory so the JSON-vs-plain-text fallback doesn't
	// double-read the source. For path == "-" (stdin), reopening returned a
	// fresh io.NopCloser around the same fd, and the discarded json.Decoder's
	// internal buffer silently swallowed the first ~512 bytes of plain-text
	// input. Buffering the whole input first removes that footgun.
	r, err := openFileOrStdin(path)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	// Try JSON array first (supports `["a", "b"]` with optional whitespace).
	var arr []string
	if jsonErr := json.NewDecoder(bytes.NewReader(data)).Decode(&arr); jsonErr == nil {
		return arr, nil
	}
	// Plain text: one name per line, # = comment.
	out := []string{}
	for _, line := range strings.Split(string(data), "\n") {
		ln := strings.TrimSpace(line)
		if ln != "" && !strings.HasPrefix(ln, "#") {
			out = append(out, ln)
		}
	}
	return out, nil
}
