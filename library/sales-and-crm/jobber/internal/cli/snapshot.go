// Copyright 2026 melanson633 and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/jobber/internal/store"
	"github.com/spf13/cobra"
)

type snapshotFinding map[string]any

func newSnapshotCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "snapshot",
		Short: "Save and diff local SQLite snapshots",
	}
	cmd.AddCommand(newSnapshotSaveCmd(flags))
	cmd.AddCommand(newSnapshotDiffCmd(flags))
	return cmd
}

func newSnapshotSaveCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var force bool
	cmd := &cobra.Command{
		Use:   "save <label>",
		Short: "Copy the current local DB into a named snapshot",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("jobber-pp-cli")
			}
			if _, err := os.Stat(dbPath); err != nil {
				return fmt.Errorf("checking source database: %w", err)
			}
			target, err := snapshotPath(dbPath, args[0])
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
				return fmt.Errorf("creating snapshot directory: %w", err)
			}
			if !force {
				if _, err := os.Stat(target); err == nil {
					return fmt.Errorf("snapshot %q already exists; use --force to overwrite", args[0])
				} else if !os.IsNotExist(err) {
					return fmt.Errorf("checking snapshot target: %w", err)
				}
			}
			in, err := os.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening source database: %w", err)
			}
			defer in.Close()
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
			if err != nil {
				return fmt.Errorf("opening snapshot target: %w", err)
			}
			if _, err := io.Copy(out, in); err != nil {
				out.Close()
				return fmt.Errorf("copying snapshot: %w", err)
			}
			if err := out.Sync(); err != nil {
				out.Close()
				return fmt.Errorf("syncing snapshot: %w", err)
			}
			if err := out.Close(); err != nil {
				return fmt.Errorf("closing snapshot: %w", err)
			}
			if flags.asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{"event": "snapshot_save", "label": args[0], "path": target})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Saved snapshot %s\t%s\n", args[0], target)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite an existing snapshot")
	return cmd
}

func newSnapshotDiffCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int
	cmd := &cobra.Command{
		Use:   "diff <label-a> <label-b>",
		Short: "Diff two saved local DB snapshots",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("jobber-pp-cli")
			}
			aPath, err := snapshotPath(dbPath, args[0])
			if err != nil {
				return err
			}
			bPath, err := snapshotPath(dbPath, args[1])
			if err != nil {
				return err
			}
			a, err := store.OpenReadOnly(aPath)
			if err != nil {
				return fmt.Errorf("opening snapshot %q: %w", args[0], err)
			}
			defer a.Close()
			b, err := store.OpenReadOnly(bPath)
			if err != nil {
				return fmt.Errorf("opening snapshot %q: %w", args[1], err)
			}
			defer b.Close()

			findings, summary, err := diffSnapshots(a, b, args[0], args[1], limit)
			if err != nil {
				return err
			}
			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				for _, f := range findings {
					if err := enc.Encode(f); err != nil {
						return err
					}
				}
				return enc.Encode(summary)
			}
			printSnapshotHuman(cmd, findings, summary)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum rows per section")
	return cmd
}

func diffSnapshots(a, b *store.Store, labelA, labelB string, limit int) ([]snapshotFinding, snapshotFinding, error) {
	clientsA, err := loadSnapshotClients(a)
	if err != nil {
		return nil, nil, fmt.Errorf("loading clients from %s: %w", labelA, err)
	}
	clientsB, err := loadSnapshotClients(b)
	if err != nil {
		return nil, nil, fmt.Errorf("loading clients from %s: %w", labelB, err)
	}
	invoicesA, err := loadSnapshotInvoices(a)
	if err != nil {
		return nil, nil, fmt.Errorf("loading invoices from %s: %w", labelA, err)
	}
	invoicesB, err := loadSnapshotInvoices(b)
	if err != nil {
		return nil, nil, fmt.Errorf("loading invoices from %s: %w", labelB, err)
	}

	var findings []snapshotFinding
	newClients, statusChanges, newlyPaid := 0, 0, 0
	var arDeltaTotal float64
	sectionCount := map[string]int{}
	add := func(section string, f snapshotFinding) {
		if limit > 0 && sectionCount[section] >= limit {
			return
		}
		sectionCount[section]++
		findings = append(findings, f)
	}

	for id, cb := range clientsB {
		ca, exists := clientsA[id]
		if !exists {
			newClients++
			add("new", snapshotFinding{"event": "snapshot_diff_new_client", "client_id": id, "name": cb.Name})
			continue
		}
		if ca.IsLead != cb.IsLead {
			statusChanges++
			add("status", snapshotFinding{"event": "snapshot_diff_status", "client_id": id, "field": "is_lead", "from": ca.IsLead, "to": cb.IsLead})
		}
		if ca.IsArchived != cb.IsArchived {
			statusChanges++
			add("status", snapshotFinding{"event": "snapshot_diff_status", "client_id": id, "field": "is_archived", "from": ca.IsArchived, "to": cb.IsArchived})
		}
		delta := cb.Balance - ca.Balance
		if math.Abs(delta) > 0.005 {
			arDeltaTotal += delta
			add("ar", snapshotFinding{"event": "snapshot_diff_ar_delta", "client_id": id, "name": cb.Name, "balance_a": ca.Balance, "balance_b": cb.Balance, "delta": delta})
		}
	}
	for id, ib := range invoicesB {
		ia, exists := invoicesA[id]
		if !exists {
			continue
		}
		if !ia.Paid && ib.Paid {
			newlyPaid++
			add("paid", snapshotFinding{"event": "snapshot_diff_paid", "invoice_id": id, "invoice_number": ib.Number, "client_id": ib.ClientID, "total": ib.Total, "payments_total_a": ia.PaymentsTotal, "payments_total_b": ib.PaymentsTotal})
		}
	}
	summary := snapshotFinding{"event": "snapshot_diff_summary", "label_a": labelA, "label_b": labelB, "new_client_count": newClients, "status_change_count": statusChanges, "newly_paid_count": newlyPaid, "ar_delta_total": arDeltaTotal}
	return findings, summary, nil
}

type snapshotClient struct {
	Name       string
	IsLead     bool
	IsArchived bool
	Balance    float64
}

func loadSnapshotClients(s *store.Store) (map[string]snapshotClient, error) {
	rows, err := s.Query(`SELECT id, COALESCE(name, company_name, trim(COALESCE(first_name, '') || ' ' || COALESCE(last_name, '')), ''), COALESCE(is_lead, 0), COALESCE(is_archived, 0), COALESCE(balance, 0) FROM clients`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]snapshotClient{}
	for rows.Next() {
		var id string
		var c snapshotClient
		var lead, archived int
		if err := rows.Scan(&id, &c.Name, &lead, &archived, &c.Balance); err != nil {
			return nil, err
		}
		c.IsLead = lead != 0
		c.IsArchived = archived != 0
		out[id] = c
	}
	return out, rows.Err()
}

type snapshotInvoice struct {
	Number        string
	ClientID      string
	Total         float64
	PaymentsTotal float64
	Paid          bool
}

func loadSnapshotInvoices(s *store.Store) (map[string]snapshotInvoice, error) {
	rows, err := s.Query(`SELECT id, COALESCE(invoice_number, ''), COALESCE(json_extract(data, '$.client.id'), ''), COALESCE(invoice_status, ''), COALESCE(total, 0), COALESCE(payments_total, 0) FROM invoices`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]snapshotInvoice{}
	for rows.Next() {
		var id, status string
		var inv snapshotInvoice
		if err := rows.Scan(&id, &inv.Number, &inv.ClientID, &status, &inv.Total, &inv.PaymentsTotal); err != nil {
			return nil, err
		}
		inv.Paid = status == "paid" || inv.PaymentsTotal >= inv.Total
		out[id] = inv
	}
	return out, rows.Err()
}

func snapshotPath(dbPath, label string) (string, error) {
	if label == "" || filepath.Base(label) != label {
		return "", fmt.Errorf("snapshot label must be a simple file name")
	}
	return filepath.Join(filepath.Dir(dbPath), "snapshots", label+".db"), nil
}

func printSnapshotHuman(cmd *cobra.Command, findings []snapshotFinding, summary snapshotFinding) {
	sections := []struct {
		event  string
		title  string
		fields []string
	}{
		{"snapshot_diff_new_client", "NEW CLIENTS", []string{"client_id", "name"}},
		{"snapshot_diff_status", "STATUS CHANGES", []string{"client_id", "field", "from", "to"}},
		{"snapshot_diff_paid", "NEWLY PAID INVOICES", []string{"invoice_id", "invoice_number", "client_id", "total", "payments_total_a", "payments_total_b"}},
		{"snapshot_diff_ar_delta", "OPEN AR DELTAS", []string{"client_id", "name", "balance_a", "balance_b", "delta"}},
	}
	for _, section := range sections {
		fmt.Fprintln(cmd.OutOrStdout(), section.title)
		fmt.Fprintln(cmd.OutOrStdout(), stringsJoinTab(section.fields))
		for _, f := range findings {
			if f["event"] != section.event {
				continue
			}
			values := make([]string, 0, len(section.fields))
			for _, field := range section.fields {
				values = append(values, fmt.Sprint(f[field]))
			}
			fmt.Fprintln(cmd.OutOrStdout(), stringsJoinTab(values))
		}
		fmt.Fprintln(cmd.OutOrStdout())
	}
	fmt.Fprintln(cmd.OutOrStdout(), "SUMMARY")
	fmt.Fprintln(cmd.OutOrStdout(), "label_a\tlabel_b\tnew_client_count\tstatus_change_count\tnewly_paid_count\tar_delta_total")
	fmt.Fprintf(cmd.OutOrStdout(), "%v\t%v\t%v\t%v\t%v\t%v\n", summary["label_a"], summary["label_b"], summary["new_client_count"], summary["status_change_count"], summary["newly_paid_count"], summary["ar_delta_total"])
}

func stringsJoinTab(values []string) string {
	return strings.Join(values, "\t")
}
