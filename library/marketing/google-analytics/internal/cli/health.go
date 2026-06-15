package cli

import (
	"context"
	"errors"
	"os"

	"github.com/mvanhorn/printing-press-library/library/marketing/google-analytics/internal/ga4"
	"github.com/spf13/cobra"
)

func newHealthCmd(flags *rootFlags) *cobra.Command {
	var props string
	c := &cobra.Command{Use: "health", Short: "Verify credentials, Admin API, and per-property GA4 access grants", RunE: func(cmd *cobra.Command, args []string) error { return runHealth(cmd, flags, props) }}
	c.Flags().StringVar(&props, "properties", "", "Comma-separated GA4 property IDs to check (or GA4_PROPERTY_IDS)")
	return c
}
func newDoctorCmd(flags *rootFlags) *cobra.Command {
	var props string
	c := &cobra.Command{Use: "doctor", Short: "Verify credentials, Admin API, and per-property GA4 access grants", RunE: func(cmd *cobra.Command, args []string) error { return runHealth(cmd, flags, props) }}
	c.Flags().StringVar(&props, "properties", "", "Comma-separated GA4 property IDs to check (or GA4_PROPERTY_IDS)")
	return c
}
func runHealth(cmd *cobra.Command, flags *rootFlags, props string) error {
	cl, key, err := flags.newClient()
	res := map[string]any{"credential_path": credentialPath(flags), "scope": ga4.AnalyticsReadonlyScope}
	if key.ClientEmail != "" {
		res["service_account"] = key.ClientEmail
		res["project_id"] = key.ProjectID
	}
	if err != nil {
		res["ok"] = false
		res["status"] = "creds_invalid"
		res["error"] = err.Error()
		return output(cmd, flags, res, "")
	}
	summaries, status, err := cl.AccountSummaries(context.Background())
	res["visible_properties"] = visibleProperties(summaries)
	res["admin_api_status"] = status
	if err != nil {
		res["ok"] = false
		res["status"] = "api_or_token_error"
		res["error"] = err.Error()
		return output(cmd, flags, res, "")
	}
	targets := splitCSV(props)
	if len(targets) == 0 {
		targets = append(targets, configuredProperty(flags))
		if env := os.Getenv("GA4_PROPERTY_IDS"); env != "" {
			targets = append(targets, splitCSV(env)...)
		}
	}
	targets = uniqNonEmpty(targets)
	checks := []map[string]any{}
	allOK := true
	for _, p := range targets {
		_, st, e := cl.RunReport(context.Background(), p, reportRequest("sessions", "", "7daysAgo", "yesterday", 1))
		chk := map[string]any{"property": p, "status_code": st, "ok": e == nil}
		if e != nil {
			allOK = false
			chk["error"] = classifyAccessError(e)
			chk["detail"] = e.Error()
		}
		checks = append(checks, chk)
	}
	res["property_checks"] = checks
	if len(targets) == 0 {
		res["ok"] = true
		res["status"] = "token_valid_no_property_requested"
	} else if allOK {
		res["ok"] = true
		res["status"] = "valid"
	} else {
		res["ok"] = false
		res["status"] = "property_not_shared_or_invalid"
	}
	return output(cmd, flags, res, renderHealth(res))
}
func classifyAccessError(err error) string {
	var ae ga4.APIError
	if errors.As(err, &ae) {
		if ae.Status == 401 {
			return "creds_invalid_or_token_rejected"
		}
		if ae.Status == 403 {
			return "api_enabled_but_property_not_shared_or_permission_denied"
		}
		if ae.Status == 404 {
			return "property_not_found_or_not_shared"
		}
	}
	return "request_failed"
}
