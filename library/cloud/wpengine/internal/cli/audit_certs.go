// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: fleet-wide SSL certificate expiry audit.
// pp:data-source local

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/cloud/wpengine/internal/cliutil"
)

type auditCertRow struct {
	Install       string   `json:"install"`
	InstallID     string   `json:"install_id"`
	Environment   string   `json:"environment,omitempty"`
	CommonName    string   `json:"common_name"`
	Domains       []string `json:"domains,omitempty"`
	ExpiresTime   string   `json:"expires_time"`
	DaysLeft      int      `json:"days_left"`
	Expired       bool     `json:"expired"`
	Status        string   `json:"status,omitempty"`
	CertSource    string   `json:"cert_source,omitempty"`
	Wildcard      bool     `json:"wildcard,omitempty"`
	AutoRenew     bool     `json:"auto_renew,omitempty"`
	CertificateID string   `json:"certificate_id"`
}

// auditCertsView mirrors the audit versions/domains envelope convention:
// results plus a scanned denominator, so JSON consumers can distinguish a
// clean fleet (scanned > 0, expiring empty) from an unsynced or empty
// mirror (scanned == 0).
type auditCertsView struct {
	Expiring            []auditCertRow `json:"expiring"`
	ScannedCertificates int            `json:"scanned_certificates"`
}

func newNovelAuditCertsCmd(flags *rootFlags) *cobra.Command {
	var flagExpiring string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "certs",
		Short: "See every SSL certificate across your whole fleet sorted by days to expiry",
		Long: strings.Trim(`
Use this command for fleet-wide certificate expiry windows: it joins synced
installs and SSL certificates in the local mirror, computes days to expiry,
and lists everything expiring inside the window (already-expired certs
included), sorted soonest first.

Do NOT use it to find domains with no certificate at all or unverified
domains; use 'audit domains' instead.

Requires a synced mirror: run 'wpengine-pp-cli sync' first.
`, "\n"),
		Example: strings.Trim(`
  # Certificates expiring in the next 30 days, fleet-wide
  wpengine-pp-cli audit certs --expiring 30d

  # Agent-friendly JSON, next 90 days
  wpengine-pp-cli audit certs --expiring 90d --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would scan local mirror for certificates expiring within", flagExpiring)
				return nil
			}
			if err := rejectLiveDataSource(flags); err != nil {
				return err
			}
			window, err := cliutil.ParseDurationLoose(flagExpiring)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid --expiring %q: %w (use forms like 30d, 12w, 720h)", flagExpiring, err))
			}
			db, handled, err := openAuditStore(cmd, flags, dbPath, "ssl_certificates", `{"expiring":[],"scanned_certificates":0}`)
			if err != nil || handled {
				return err
			}
			defer db.Close()

			ctx := cmd.Context()

			// Drain-first: scan certificates joined to install identity in one
			// result set, then post-process in Go.
			rows, err := db.DB().QueryContext(ctx, `
				SELECT c.id, c.installs_id, c.data,
				       COALESCE(i.name, ''), COALESCE(i.environment, '')
				FROM ssl_certificates c
				LEFT JOIN installs i ON i.id = c.installs_id`)
			if err != nil {
				return fmt.Errorf("querying certificates: %w", err)
			}
			type rawCert struct {
				id, installID, installName, environment string
				data                                    []byte
			}
			raws := make([]rawCert, 0)
			if err := drainRows(rows, "certificate", func(rs *sql.Rows) error {
				var r rawCert
				if err := rs.Scan(&r.id, &r.installID, &r.data, &r.installName, &r.environment); err != nil {
					return err
				}
				raws = append(raws, r)
				return nil
			}); err != nil {
				return err
			}

			now := time.Now().UTC()
			cutoff := now.Add(window)
			results := make([]auditCertRow, 0)
			for _, r := range raws {
				var cert struct {
					ID          json.Number `json:"id"`
					CommonName  string      `json:"common_name"`
					Domains     []string    `json:"domains"`
					ExpiresTime string      `json:"expires_time"`
					Status      string      `json:"status"`
					CertSource  string      `json:"cert_source"`
					Wildcard    bool        `json:"wildcard"`
					AutoRenew   bool        `json:"auto_renew"`
				}
				if err := json.Unmarshal(r.data, &cert); err != nil {
					continue
				}
				// Prefer the API's own certificate id; the store row id is a
				// composite sync key (contains a NUL separator) not meant for
				// user-facing output.
				certID := cert.ID.String()
				if certID == "" {
					if head, _, found := strings.Cut(r.id, "\x00"); found {
						certID = head
					} else {
						certID = r.id
					}
				}
				exp, ok := parseAPITime(cert.ExpiresTime)
				if !ok {
					continue
				}
				if exp.After(cutoff) {
					continue
				}
				results = append(results, auditCertRow{
					Install:       r.installName,
					InstallID:     r.installID,
					Environment:   r.environment,
					CommonName:    cert.CommonName,
					Domains:       cert.Domains,
					ExpiresTime:   exp.UTC().Format(time.RFC3339),
					DaysLeft:      int(math.Floor(time.Until(exp).Hours() / 24)),
					Expired:       exp.Before(now),
					Status:        cert.Status,
					CertSource:    cert.CertSource,
					Wildcard:      cert.Wildcard,
					AutoRenew:     cert.AutoRenew,
					CertificateID: certID,
				})
			}
			sort.Slice(results, func(i, j int) bool { return results[i].ExpiresTime < results[j].ExpiresTime })

			if len(results) == 0 && !flags.asJSON && !flags.agent {
				fmt.Fprintf(cmd.OutOrStdout(), "no certificates expiring within %s (scanned %d certificates)\n", flagExpiring, len(raws))
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), auditCertsView{
				Expiring:            results,
				ScannedCertificates: len(raws),
			}, flags)
		},
	}
	cmd.Flags().StringVar(&flagExpiring, "expiring", "30d", "Expiry window (e.g. 30d, 12w, 720h); certs expiring inside it are reported")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: local data dir)")
	return cmd
}
