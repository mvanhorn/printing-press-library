// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto

package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/uber-jobs/internal/store"
	"github.com/mvanhorn/printing-press-library/library/productivity/uber-jobs/internal/uberjobs"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newNovelGetCmd(flags))
	})
}

func newNovelGetCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Fetch one Uber posting by its id and exit not-found when the site no longer lists it",
		Long: strings.Trim(`
Fetch one Uber careers posting by its requisition id, the number in its job URL

The careers site has no single-posting endpoint, so a live get reads the whole
corpus (a size probe plus one full page, two requests; gets within two minutes
reuse that response) and keeps only the row whose id equals the requested id.
No exact match exits not-found (3). A partial read that misses the id exits 5
instead, because absence from a partial read proves nothing.

When the careers site refuses, the posting comes from Uber's Oracle
candidate-experience API (one request) and meta.source says oracle-ce. With
--data-source local the local store answers; a posting the last complete sync
marked closed exits not-found there too.

Exit codes: 2 usage, 3 not found, 5 API or content error, 6 DNS or transport
failure, 7 refused (403, 429, or a challenge; never retried).`, "\n"),
		Example: strings.Trim(`
  uber-jobs-pp-cli get 301235 --json
  uber-jobs-pp-cli get 301235 --data-source local --agent`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "auto",
			"pp:happy-args":  "id=301235",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			ds, err := resolveDataSource(cmd, flags, "auto", "live", "local")
			if err != nil {
				return err
			}
			if len(args) > 1 {
				return usageErr(fmt.Errorf("get takes one posting id, got %d; use check for several ids", len(args)))
			}
			id := ""
			if len(args) == 1 {
				id = strings.TrimSpace(args[0])
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, describeGet(flags, ds, dbPath, id))
			}
			if id == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a posting id is required, for example: uber-jobs-pp-cli get 301235"))
			}
			if !validPostingID(id) {
				return usageErr(fmt.Errorf("%q is not a posting id: ids are the digits in a jobs.uber.com/en/jobs/<id>/ URL", id))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			env, p, err := getPosting(ctx, flags, ds, dbPath, id)
			if err != nil {
				return err
			}
			return printEnvelope(cmd, flags, env, func(w io.Writer) error {
				return renderPostingDetail(w, env, p)
			})
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store path (default: the CLI data dir)")
	addDataSourceFlag(cmd, "auto", "Where to read: auto (live, then the local store if the site is unreachable), live, or local")
	return cmd
}

// describeGet renders the real request a get would send, for --dry-run.
func describeGet(flags *rootFlags, ds, dbPath, id string) string {
	if id == "" {
		id = "<id>"
	}
	if ds == "local" {
		return fmt.Sprintf("read posting %s from the local store at %s", id, uberDBPath(dbPath))
	}
	desc, err := describeRead(baseURLFor(flags), uberjobs.Filters{})
	if err != nil {
		return "read posting " + id
	}
	return fmt.Sprintf("%s and keep only id %s; on a refusal, read it from the Oracle candidate-experience API", desc, id)
}

// getPosting answers one id from the chosen source. auto reads live (with
// the Oracle fallback on a refusal) and uses the local store only when both
// live paths fail and the store holds the posting as open.
func getPosting(ctx context.Context, flags *rootFlags, ds, dbPath, id string) (uberjobs.Envelope, *uberjobs.Posting, error) {
	env := uberjobs.NewEnvelope("get", "", []uberjobs.Posting{}, 0)
	if ds == "local" {
		return getLocal(ctx, dbPath, id, env)
	}
	c, err := newUberClient(flags)
	if err != nil {
		return env, nil, err
	}
	p, lerr := getLive(ctx, c, id, &env)
	if lerr == nil {
		env.Results = []uberjobs.Posting{*p}
		env.Hits, env.Returned = 1, 1
		return env, p, nil
	}
	var nf *uberjobs.NotFoundError
	if errors.As(lerr, &nf) {
		return env, nil, notFoundErr(lerr)
	}
	if ds == "auto" && (uberjobs.IsRefusal(lerr) || uberjobs.IsTransport(lerr)) {
		if lenv, lp, gerr := getLocal(ctx, dbPath, id, uberjobs.NewEnvelope("get", "", []uberjobs.Posting{}, 0)); gerr == nil {
			lenv.Meta.Fallback = true
			lenv.Meta.FallbackFrom = lerr.Error()
			lenv.Meta.Note = "the live read failed, so this row comes from the local store; " + lenv.Meta.Note
			return lenv, lp, nil
		}
	}
	return env, nil, uberErr(lerr)
}

// getLive reads the corpus once and returns the row whose id is exactly id.
func getLive(ctx context.Context, c *uberjobs.Client, id string, env *uberjobs.Envelope) (*uberjobs.Posting, error) {
	res, err := c.SearchAll(ctx, uberjobs.Query{})
	if err == nil {
		env.Meta.Source = uberjobs.SourceSite
		env.Meta.Requests = c.Requests()
		env.Meta.Cache = res.Cache
		env.Meta.Complete = res.Complete
		env.Scanned = len(res.Rows)
		env.ScanCapHit = res.ScanCapHit
		for _, r := range res.Rows {
			if strings.TrimSpace(string(r.ID)) != id {
				continue
			}
			p := uberjobs.Normalize(r, c.BaseURL)
			if p.ID == id {
				return &p, nil
			}
		}
		if !res.Complete || res.ScanCapHit {
			return nil, &uberjobs.ContentError{URL: c.BaseURL + "/api/jobs/search/", Reason: fmt.Sprintf("posting %s was not in a partial read (%s); absence from a partial read is not proof it closed", id, res.Note)}
		}
		return nil, &uberjobs.NotFoundError{ID: id}
	}
	if !uberjobs.IsRefusal(err) || !c.OracleEnabled() {
		return nil, err
	}
	d, oerr := c.OracleGet(ctx, id)
	if oerr != nil {
		var nf *uberjobs.NotFoundError
		if errors.As(oerr, &nf) {
			return nil, oerr
		}
		return nil, fmt.Errorf("%w; the Oracle fallback also failed: %v", err, oerr)
	}
	p := uberjobs.NormalizeOracleDetail(d, c.BaseURL)
	if p.ID != id {
		return nil, &uberjobs.NotFoundError{ID: id}
	}
	env.Meta.Source = uberjobs.SourceOracle
	env.Meta.Fallback = true
	env.Meta.FallbackFrom = err.Error()
	env.Meta.Complete = true
	env.Meta.Requests = c.Requests()
	env.Meta.Note = "the careers site refused, so this posting comes from Uber's Oracle candidate-experience API; job_category is null in this mode"
	env.Scanned = 1
	return &p, nil
}

// getLocal answers from the local store, read-only: a missing store is an
// empty one and is never created. A closed posting is not-found.
func getLocal(ctx context.Context, dbPath, id string, env uberjobs.Envelope) (uberjobs.Envelope, *uberjobs.Posting, error) {
	var stored []uberjobs.StoredPosting
	var last *uberjobs.SyncRun
	_, err := withStoreRO(ctx, dbPath, func(s *store.Store) error {
		var err error
		if stored, err = uberjobs.LoadPostings(ctx, s.DB(), false); err != nil {
			return err
		}
		last, err = uberjobs.LastFullSync(ctx, s.DB())
		return err
	})
	if err != nil && !isMissingTable(err) {
		return env, nil, err
	}
	env.Meta.Source = uberjobs.SourceLocal
	env.Meta.Complete = last != nil
	env.Scanned = len(stored)
	for i := range stored {
		sp := stored[i]
		if sp.ID != id {
			continue
		}
		if sp.ClosedOn != nil {
			return env, nil, notFoundErr(fmt.Errorf("posting %s closed on %s according to the local store", id, *sp.ClosedOn))
		}
		env.Results = []uberjobs.StoredPosting{sp}
		env.Hits, env.Returned = 1, 1
		env.Meta.Note = localFirstNote("local", last)
		return env, &sp.Posting, nil
	}
	return env, nil, notFoundErr(fmt.Errorf("posting %s is not in the local store (%d postings); run sync, or use --data-source live", id, len(stored)))
}

// renderPostingDetail is the human view of one posting.
func renderPostingDetail(w io.Writer, env uberjobs.Envelope, p *uberjobs.Posting) error {
	posted := deref(p.PostedOn, "-")
	if p.PostedDateIsFloor {
		posted = "unknown (the site shows its migration date " + deref(p.PostedRaw, "") + ")"
	}
	fmt.Fprintf(w, "%s  %s\n", p.ID, p.Title)
	fmt.Fprintf(w, "posted:    %s\n", posted)
	fmt.Fprintf(w, "team:      %s / %s\n", deref(p.JobCategory, "-"), deref(p.SubTeam, "-"))
	locs := make([]string, 0, len(p.Locations))
	for _, l := range p.Locations {
		if s := deref(l.Address, deref(joinLocationText(l), "")); s != "" {
			locs = append(locs, s)
		}
	}
	fmt.Fprintf(w, "locations: %s\n", strings.Join(locs, "; "))
	fmt.Fprintf(w, "url:       %s\n", p.URL)
	if p.SalaryText != nil {
		fmt.Fprintf(w, "pay:       %s\n", *p.SalaryText)
	}
	fmt.Fprintf(w, "source:    %s\n\n", env.Meta.Source)
	fmt.Fprintln(w, deref(p.Description, "(no description)"))
	if env.Meta.Note != "" {
		fmt.Fprintln(w, "\nnote:", env.Meta.Note)
	}
	return nil
}

func joinLocationText(l uberjobs.Location) *string {
	parts := make([]string, 0, 3)
	for _, v := range []*string{l.City, l.Region, l.Country} {
		if v != nil && *v != "" {
			parts = append(parts, *v)
		}
	}
	if len(parts) == 0 {
		return nil
	}
	s := strings.Join(parts, ", ")
	return &s
}
