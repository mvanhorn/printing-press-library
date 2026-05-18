// Copyright 2026 ChrisDrit. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newOwnerCmd() *cobra.Command {
	var (
		exact    bool
		limit    int
		withHist bool
	)
	cmd := &cobra.Command{
		Use:   "owner <name>",
		Short: "List aircraft for a registered owner name (substring or exact).",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runOwner(cmd.Context(), args[0], exact, limit, withHist)
		},
	}
	cmd.Flags().BoolVar(&exact, "exact", false, "Match the owner name exactly (case-insensitive); default is substring")
	cmd.Flags().IntVar(&limit, "limit", 500, "Maximum aircraft to return")
	cmd.Flags().BoolVar(&withHist, "with-history", false, "Add an accident_count column per aircraft (extra join, slightly slower)")
	return cmd
}

type OwnerHit struct {
	Registration  string  `json:"registration"`
	OwnerName     string  `json:"owner_name"`
	YearMfr       *int    `json:"year_mfr,omitempty"`
	StatusCode    *string `json:"status_code,omitempty"`
	MakeModelCode *string `json:"make_model_code,omitempty"`
	Manufacturer  *string `json:"manufacturer,omitempty"`
	Model         *string `json:"model,omitempty"`
	AccidentCount *int    `json:"accident_count,omitempty"`
}

func runOwner(ctx context.Context, raw string, exact bool, limit int, withHist bool) error {
	dbPath, st, err := openReadStore(ctx)
	if err != nil {
		return err
	}
	defer st.Close()

	name := strings.TrimSpace(raw)
	if name == "" {
		return fmt.Errorf("owner name must not be empty")
	}
	if limit <= 0 {
		limit = 500
	}

	var rows *sql.Rows
	pattern := name
	op := "LIKE"
	if exact {
		op = "="
	} else {
		pattern = "%" + name + "%"
	}

	q := `SELECT a.registration, a.owner_name, a.year_mfr, a.status_code,
		a.make_model_code, mm.manufacturer, mm.model`
	if withHist {
		q += `, (SELECT COUNT(*) FROM event_aircraft ea WHERE ea.registration = a.registration) AS accident_count`
	} else {
		q += `, NULL AS accident_count`
	}
	q += ` FROM aircraft a
		LEFT JOIN make_model mm ON mm.code = a.make_model_code
		WHERE a.owner_name ` + op + ` ? COLLATE NOCASE
		ORDER BY a.owner_name COLLATE NOCASE, a.registration
		LIMIT ?`

	rows, err = st.DB().QueryContext(ctx, q, pattern, limit+1)
	if err != nil {
		return fmt.Errorf("query owner %s: %w", name, err)
	}
	defer rows.Close()

	var hits []OwnerHit
	truncated := false
	for rows.Next() {
		if len(hits) == limit {
			truncated = true
			break
		}
		var h OwnerHit
		var year, count sql.NullInt64
		var status, mmcode, mfr, model sql.NullString
		var owner sql.NullString
		if err := rows.Scan(&h.Registration, &owner, &year, &status, &mmcode, &mfr, &model, &count); err != nil {
			return err
		}
		h.OwnerName = owner.String
		if year.Valid {
			v := int(year.Int64)
			h.YearMfr = &v
		}
		h.StatusCode = nullToPtr(status)
		h.MakeModelCode = nullToPtr(mmcode)
		h.Manufacturer = nullToPtr(mfr)
		h.Model = nullToPtr(model)
		if count.Valid {
			v := int(count.Int64)
			h.AccidentCount = &v
		}
		hits = append(hits, h)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	rc := len(hits)
	env := Envelope{
		Meta: Meta{
			Source:    "local",
			DBPath:    dbPath,
			SyncedAt:  latestSyncedAt(ctx, st),
			Query:     map[string]any{"owner": name, "exact": exact, "limit": limit, "with_history": withHist},
			RowCount:  &rc,
			Truncated: truncated,
		},
		Results: hits,
	}

	if flagJSON || flagSelect != "" {
		return emitEnvelope(env)
	}
	return renderOwnerText(name, hits, truncated, withHist)
}

func renderOwnerText(name string, hits []OwnerHit, truncated, withHist bool) error {
	if len(hits) == 0 {
		fmt.Printf("No aircraft found for owner matching %q.\n", name)
		return nil
	}
	fmt.Printf("Aircraft owned by names matching %q (%d shown%s)\n\n", name, len(hits), truncatedSuffix(truncated))
	for _, h := range hits {
		typeStr := ""
		if h.Manufacturer != nil && h.Model != nil {
			typeStr = *h.Manufacturer + " " + *h.Model
		} else if h.MakeModelCode != nil {
			typeStr = "code=" + *h.MakeModelCode
		}
		year := ""
		if h.YearMfr != nil {
			year = fmt.Sprintf(" (%d)", *h.YearMfr)
		}
		extra := ""
		if withHist && h.AccidentCount != nil && *h.AccidentCount > 0 {
			extra = fmt.Sprintf("  [NTSB events: %d]", *h.AccidentCount)
		}
		fmt.Printf("  %s  %-50s  %s%s%s\n", h.Registration, truncateStr(h.OwnerName, 50), typeStr, year, extra)
	}
	return nil
}

func truncatedSuffix(t bool) string {
	if t {
		return ", more available — raise --limit"
	}
	return ""
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
