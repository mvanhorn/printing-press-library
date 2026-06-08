// Copyright 2026 "Chris Carson" and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/monitoring/ninjaone/internal/cliutil"

	"github.com/spf13/cobra"
)

// pp:data-source live

// alertPredicate is the parsed form of the --where flag (comma-separated
// clauses ANDed together).
type alertPredicate struct {
	ageOp     string // ">" or "<" or ""
	ageSecs   int64
	org       string // name substring or numeric id
	condition string // condition name substring
	severity  string
}

// parseAlertPredicate parses clauses like "age>72h,org=Acme,condition=cpu".
func parseAlertPredicate(s string) (alertPredicate, error) {
	var p alertPredicate
	for _, clause := range strings.Split(s, ",") {
		clause = strings.TrimSpace(clause)
		if clause == "" {
			continue
		}
		switch {
		case strings.HasPrefix(clause, "age>"), strings.HasPrefix(clause, "age<"):
			op := string(clause[3])
			d, err := cliutil.ParseDurationLoose(clause[4:])
			if err != nil {
				return p, fmt.Errorf("invalid age duration in %q: %w", clause, err)
			}
			p.ageOp = op
			p.ageSecs = int64(d.Seconds())
		case strings.HasPrefix(clause, "org="):
			p.org = strings.TrimSpace(clause[4:])
		case strings.HasPrefix(clause, "condition="):
			p.condition = strings.TrimSpace(clause[len("condition="):])
		case strings.HasPrefix(clause, "severity="):
			p.severity = strings.TrimSpace(clause[len("severity="):])
		default:
			return p, fmt.Errorf("unrecognized predicate clause %q (supported: age>DUR, age<DUR, org=, condition=, severity=)", clause)
		}
	}
	return p, nil
}

// matches reports whether an alert satisfies the predicate. nowSec is the
// current epoch seconds; orgName is the resolved org name for the alert's
// device.
func (p alertPredicate) matches(a njAlert, orgID int64, orgName string, nowSec int64) bool {
	if p.ageOp != "" {
		age := nowSec - a.createSeconds()
		if p.ageOp == ">" && !(age > p.ageSecs) {
			return false
		}
		if p.ageOp == "<" && !(age < p.ageSecs) {
			return false
		}
	}
	if p.org != "" && !orgMatches(p.org, orgID, orgName) {
		return false
	}
	if p.condition != "" && !strings.Contains(strings.ToLower(a.ConditionName), strings.ToLower(p.condition)) {
		return false
	}
	if p.severity != "" && !strings.EqualFold(a.Severity, p.severity) {
		return false
	}
	return true
}

type alertClearFailure struct {
	UID   string `json:"uid"`
	Error string `json:"error"`
}

type alertClearView struct {
	Where      string              `json:"where"`
	Applied    bool                `json:"applied"`
	MatchCount int                 `json:"matchCount"`
	Uids       []string            `json:"uids"`
	Succeeded  int                 `json:"succeeded"`
	Failures   []alertClearFailure `json:"failures"`
	Unresolved int                 `json:"unresolved_device_orgs,omitempty"`
	Note       string              `json:"note,omitempty"`
}

func newNovelAlertClearCmd(flags *rootFlags) *cobra.Command {
	var (
		flagWhere string
		flagApply bool
		flagLimit int
	)

	cmd := &cobra.Command{
		Use:   "alert-clear",
		Short: "Reset every alert matching a predicate (condition, organization, or age) in one throttled, dry-run-first operation.",
		Long: `Fetch active alerts, select the ones matching a --where predicate, and (with
--apply) reset them one by one. The predicate is comma-separated clauses ANDed
together: age>DUR / age<DUR, org=<name|id>, condition=<substring>, severity=<x>.
Default behavior previews the match count without mutating.

Examples:
  ninjaone-pp-cli alert-clear --where "age>72h"                 # preview
  ninjaone-pp-cli alert-clear --where "org=Acme,condition=cpu" --apply
  ninjaone-pp-cli alert-clear --where "severity=critical" --apply --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) || cliutil.IsVerifyEnv() {
				return emitDryRunPreview(cmd, flags, "would fetch alerts, select those matching --where, and reset them when --apply is set")
			}

			if strings.TrimSpace(flagWhere) == "" {
				return usageErr(fmt.Errorf("--where is required (predicate selecting alerts to clear)"))
			}
			pred, err := parseAlertPredicate(flagWhere)
			if err != nil {
				return usageErr(err)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()

			orgs, err := fetchOrgs(ctx, c)
			if err != nil {
				return err
			}
			devices, _, err := fetchDevices(ctx, c, "", effectiveMaxScanPages(5))
			if err != nil {
				return err
			}
			_, devToOrg := deviceOrgIndex(devices)

			alerts, err := fetchAlerts(ctx, c)
			if err != nil {
				return err
			}

			now := time.Now().Unix()
			uids := make([]string, 0)
			unresolved := 0
			for _, a := range alerts {
				orgID, known := devToOrg[a.DeviceID]
				// An org predicate can only be evaluated when the alert's device
				// was in the (page-capped) device fetch. Count alerts whose device
				// org could not be resolved so the exclusion is observable instead
				// of silent.
				if pred.org != "" && a.DeviceID != 0 && !known {
					unresolved++
				}
				if a.UID != "" && pred.matches(a, orgID, orgs[orgID], now) {
					uids = append(uids, a.UID)
				}
			}
			if n := boundLimit(len(uids), flagLimit); n < len(uids) {
				uids = uids[:n]
			}

			view := alertClearView{
				Where:      flagWhere,
				MatchCount: len(uids),
				Uids:       uids,
				Unresolved: unresolved,
				Failures:   make([]alertClearFailure, 0),
			}
			if unresolved > 0 {
				view.Note = fmt.Sprintf("%d alert(s) skipped for org matching: their device was outside the scanned device pages; widen the device scan or omit the org filter", unresolved)
			}

			if !flagApply {
				if view.Note != "" {
					view.Note = "preview only; pass --apply to reset matching alerts. " + view.Note
				} else {
					view.Note = "preview only; pass --apply to reset matching alerts"
				}
				return emitAlertClearView(cmd, flags, view)
			}

			for _, uid := range uids {
				path := fmt.Sprintf("/v2/alert/%s/reset", uid)
				if _, status, err := c.Post(ctx, path, map[string]any{}); err != nil || status >= 400 {
					view.Failures = append(view.Failures, alertClearFailure{UID: uid, Error: postErrString(status, err)})
				} else {
					view.Succeeded++
				}
			}
			view.Applied = true
			return emitAlertClearView(cmd, flags, view)
		},
	}
	cmd.Flags().StringVar(&flagWhere, "where", "", "Predicate selecting alerts (REQUIRED), e.g. \"age>72h,org=Acme\"")
	cmd.Flags().BoolVar(&flagApply, "apply", false, "Actually reset matching alerts (required to mutate)")
	cmd.Flags().IntVar(&flagLimit, "limit", 0, "Maximum number of alerts to clear (0 = all)")
	return cmd
}

func emitAlertClearView(cmd *cobra.Command, flags *rootFlags, view alertClearView) error {
	if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
		return printJSONFiltered(cmd.OutOrStdout(), view, flags)
	}
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "Matched %d alert(s) for where=%q\n", view.MatchCount, view.Where)
	if view.Note != "" {
		fmt.Fprintln(w, view.Note)
		return nil
	}
	fmt.Fprintf(w, "Reset succeeded: %d  failures: %d\n", view.Succeeded, len(view.Failures))
	for _, f := range view.Failures {
		fmt.Fprintf(w, "  FAIL %s: %s\n", f.UID, f.Error)
	}
	return nil
}
