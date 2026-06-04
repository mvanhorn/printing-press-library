package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/sumble/internal/client"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/sumble/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/sumble/internal/store"

	"github.com/spf13/cobra"
)

func newStackDiffCmd(flags *rootFlags) *cobra.Command {
	var noFetch bool

	cmd := &cobra.Command{
		Use:   "stack-diff <domainA> <domainB>",
		Short: "Compare two organizations' technology stacks — shared and unique technologies",
		Long: strings.Trim(`
Compare the technology stacks of two organizations by domain. Each org's stack
is read from the local cache when available (zero credits); otherwise it is
fetched via organizations/enrich (5 credits per technology found) and cached for
next time. Pass --no-fetch to use only cached stacks and never spend credits.
`, "\n"),
		Example: strings.Trim(`
  sumble-pp-cli stack-diff stripe.com adyen.com
  sumble-pp-cli stack-diff stripe.com adyen.com --no-fetch --json
`, "\n"),
		// No mcp:read-only — without --no-fetch, stack-diff falls back to
		// organizations/enrich, which spends credits (5 per technology found).
		// It does not mutate external state, but it is not free.
		Args: cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				if len(args) == 0 {
					return cmd.Help()
				}
				return usageErr(fmt.Errorf("stack-diff needs two domains, got %d", len(args)))
			}
			domA, domB := args[0], args[1]

			db, derr := openCreditStore()
			if derr != nil {
				return configErr(derr)
			}
			defer db.Close()

			var c *client.Client
			if !noFetch && !cliutil.IsVerifyEnv() {
				cl, cerr := flags.newClient()
				if cerr != nil {
					return cerr
				}
				c = cl
			}

			techA, errA := stackForDomain(cmd.Context(), db, c, domA, flags)
			if errA != nil {
				return errA
			}
			techB, errB := stackForDomain(cmd.Context(), db, c, domB, flags)
			if errB != nil {
				return errB
			}

			shared, onlyA, onlyB := diffSets(techA, techB)

			if flags.asJSON {
				return flags.printJSON(cmd, map[string]any{
					"org_a":  domA,
					"org_b":  domB,
					"shared": shared,
					"only_a": onlyA,
					"only_b": onlyB,
				})
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Shared (%d):\n", len(shared))
			for _, t := range shared {
				fmt.Fprintf(w, "  = %s\n", t)
			}
			fmt.Fprintf(w, "Only %s (%d):\n", domA, len(onlyA))
			for _, t := range onlyA {
				fmt.Fprintf(w, "  + %s\n", t)
			}
			fmt.Fprintf(w, "Only %s (%d):\n", domB, len(onlyB))
			for _, t := range onlyB {
				fmt.Fprintf(w, "  + %s\n", t)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&noFetch, "no-fetch", false, "Use only cached stacks; never spend credits")
	return cmd
}

// stackForDomain returns the technology-name set for a domain, reading the
// local cache first and falling back to a billed enrich when a client is
// available.
func stackForDomain(ctx context.Context, db *store.Store, c *client.Client, domain string, flags *rootFlags) ([]string, error) {
	if cached, ok := readCachedStack(db.DB(), domain); ok {
		return cached, nil
	}
	if c == nil {
		// --no-fetch and nothing cached: do not return an empty stack, which
		// would render as a misleading "0 technologies" diff. Surface that the
		// domain's data is simply unavailable.
		return nil, usageErr(fmt.Errorf("no cached technology stack for %q; drop --no-fetch to fetch it (spends credits) or run it without --no-fetch first", domain))
	}
	// The enrich endpoint requires a filters object even when empty.
	raw, _, err := c.Post(ctx, "/organizations/enrich", map[string]any{
		"organization": map[string]any{"domain": domain},
		"filters":      map[string]any{},
	})
	if err != nil {
		return nil, classifyAPIError(err, flags)
	}
	env := parseEnvelope(raw)
	techs := extractTechNames(raw)
	recordEnvelope(db.DB(), "organizations.enrich", env, "stack-diff enrich "+domain)
	writeCachedStack(db.DB(), domain, techs)
	return techs, nil
}

func extractTechNames(raw json.RawMessage) []string {
	var resp struct {
		Technologies []struct {
			Name string `json:"name"`
		} `json:"technologies"`
	}
	_ = json.Unmarshal(raw, &resp)
	names := make([]string, 0, len(resp.Technologies))
	for _, t := range resp.Technologies {
		if strings.TrimSpace(t.Name) != "" {
			names = append(names, t.Name)
		}
	}
	return names
}

func readCachedStack(db *sql.DB, domain string) ([]string, bool) {
	row := db.QueryRow(`SELECT technologies FROM org_tech_cache WHERE domain = ?`, domain)
	var raw sql.NullString
	if err := row.Scan(&raw); err != nil || !raw.Valid {
		return nil, false
	}
	var techs []string
	if err := json.Unmarshal([]byte(raw.String), &techs); err != nil {
		return nil, false
	}
	return techs, true
}

func writeCachedStack(db *sql.DB, domain string, techs []string) {
	data, err := json.Marshal(techs)
	if err != nil {
		return
	}
	_, _ = db.Exec(
		`INSERT INTO org_tech_cache (domain, technologies, synced_at) VALUES (?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(domain) DO UPDATE SET technologies = excluded.technologies, synced_at = excluded.synced_at`,
		domain, string(data),
	)
}

func diffSets(a, b []string) (shared, onlyA, onlyB []string) {
	setB := make(map[string]bool, len(b))
	for _, t := range b {
		setB[t] = true
	}
	setA := make(map[string]bool, len(a))
	for _, t := range a {
		setA[t] = true
		if setB[t] {
			shared = append(shared, t)
		} else {
			onlyA = append(onlyA, t)
		}
	}
	for _, t := range b {
		if !setA[t] {
			onlyB = append(onlyB, t)
		}
	}
	sort.Strings(shared)
	sort.Strings(onlyA)
	sort.Strings(onlyB)
	return shared, onlyA, onlyB
}
