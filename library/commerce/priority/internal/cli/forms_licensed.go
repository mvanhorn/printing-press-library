// Copyright 2026 alon-auto and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: forms licensed — probe which forms are API-enabled on this
// tenant and cache the verdicts locally.

package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/priority/internal/client"
	"github.com/mvanhorn/printing-press-library/library/commerce/priority/internal/store"
)

// pp:data-source live

var defaultLicenseProbeForms = []string{
	"ORDERS", "ORDERITEMS", "CUSTOMERS", "AINVOICES", "LOGPART", "PORDERS", "SUPPLIERS", "WAREHOUSES", "FAMILY_LOG",
}

type licenseVerdict struct {
	Form      string `json:"form"`
	Verdict   string `json:"verdict"` // licensed | blocked | not-found | error | cached-*
	HTTPState int    `json:"http_status,omitempty"`
	Message   string `json:"message,omitempty"`
	CheckedAt string `json:"checked_at"`
	FromCache bool   `json:"from_cache"`
}

type licenseView struct {
	Tenant   string           `json:"tenant"`
	Verdicts []licenseVerdict `json:"verdicts"`
	Probed   int              `json:"probed_live"`
	Note     string           `json:"note,omitempty"`
}

func newNovelFormsLicensedCmd(flags *rootFlags) *cobra.Command {
	var formsCSV string
	var recheck bool
	cmd := &cobra.Command{
		Use:   "licensed",
		Short: "Discover which forms are actually API-enabled on this tenant instead of finding out via 400 errors",
		Long: strings.Trim(`
Probes each form with a throttled GET ?$top=1 and classifies the answer:
licensed (200), blocked ("API Cannot Be Run for This Form" / license 400),
not-found (404), or error. Verdicts are cached per tenant in the local store;
use --recheck to re-probe cached forms.`, "\n"),
		Example: strings.Trim(`
  priority-pp-cli forms licensed --forms ORDERS,CUSTOMERS,AINVOICES
  priority-pp-cli forms licensed --recheck
  priority-pp-cli forms licensed --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--forms=ORDERS"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would probe form API licensing with throttled $top=1 requests")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			tenant := tenantKeyFromClient(c)
			db, err := store.OpenWithContext(ctx, defaultDBPath("priority-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			probeForms := defaultLicenseProbeForms
			if strings.TrimSpace(formsCSV) != "" {
				probeForms = nil
				for _, f := range strings.Split(formsCSV, ",") {
					f = strings.ToUpper(strings.TrimSpace(f))
					if f != "" {
						probeForms = append(probeForms, f)
					}
				}
			}
			if len(probeForms) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--forms produced an empty form list"))
			}

			view := licenseView{Tenant: tenant}
			for _, form := range probeForms {
				if !recheck {
					var v licenseVerdict
					var status int
					err := db.DB().QueryRowContext(ctx,
						`SELECT verdict, COALESCE(http_status, 0), COALESCE(message, ''), checked_at FROM pp_license_verdicts WHERE tenant = ? AND form = ?`,
						tenant, form).Scan(&v.Verdict, &status, &v.Message, &v.CheckedAt)
					if err == nil {
						v.Form = form
						v.HTTPState = status
						v.FromCache = true
						view.Verdicts = append(view.Verdicts, v)
						continue
					}
				}
				verdict := probeFormLicense(ctx, c, form)
				verdict.CheckedAt = time.Now().UTC().Format(time.RFC3339)
				view.Probed++
				if _, err := db.DB().ExecContext(ctx,
					`INSERT INTO pp_license_verdicts (tenant, form, verdict, http_status, message, checked_at) VALUES (?, ?, ?, ?, ?, ?)
					 ON CONFLICT(tenant, form) DO UPDATE SET verdict = excluded.verdict, http_status = excluded.http_status, message = excluded.message, checked_at = excluded.checked_at`,
					tenant, form, verdict.Verdict, verdict.HTTPState, verdict.Message, verdict.CheckedAt); err != nil {
					return fmt.Errorf("caching verdict for %s: %w", form, err)
				}
				view.Verdicts = append(view.Verdicts, verdict)
			}
			if view.Probed == 0 {
				view.Note = "all verdicts served from cache; use --recheck to re-probe"
			}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			w := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(w, "FORM\tVERDICT\tSTATUS\tCACHED\tMESSAGE")
			for _, v := range view.Verdicts {
				cached := ""
				if v.FromCache {
					cached = "yes"
				}
				fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\n", v.Form, v.Verdict, v.HTTPState, cached, truncate(v.Message, 60))
			}
			if err := w.Flush(); err != nil {
				return err
			}
			if view.Note != "" {
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&formsCSV, "forms", "", "comma-separated form names to probe (default: common ERP forms)")
	cmd.Flags().BoolVar(&recheck, "recheck", false, "re-probe forms even when a cached verdict exists")
	return cmd
}

// probeFormLicense classifies one form's API accessibility from a $top=1 GET.
func probeFormLicense(ctx context.Context, c *client.Client, form string) licenseVerdict {
	v := licenseVerdict{Form: form}
	_, err := c.GetNoCache(ctx, "/"+form, map[string]string{"$top": "1"})
	if err == nil {
		v.Verdict = "licensed"
		v.HTTPState = 200
		return v
	}
	var apiErr *client.APIError
	if errors.As(err, &apiErr) {
		v.HTTPState = apiErr.StatusCode
		v.Message = truncate(apiErr.Body, 160)
		lower := strings.ToLower(apiErr.Body)
		switch {
		case apiErr.StatusCode == 404:
			v.Verdict = "not-found"
		case strings.Contains(lower, "cannot be run") || strings.Contains(lower, "license") || strings.Contains(lower, "not licensed"):
			v.Verdict = "blocked"
		case apiErr.StatusCode == 400:
			v.Verdict = "blocked"
		case apiErr.StatusCode == 401 || apiErr.StatusCode == 403:
			v.Verdict = "unauthorized"
		default:
			v.Verdict = "error"
		}
		return v
	}
	v.Verdict = "error"
	v.Message = truncate(err.Error(), 160)
	return v
}
