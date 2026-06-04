package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/ars-sicilia/internal/store"
	"github.com/spf13/cobra"
)

const createFirmatariTable = `
CREATE TABLE IF NOT EXISTS firmatari (
	nome      TEXT NOT NULL,
	legisl    TEXT NOT NULL,
	isis_expr TEXT NOT NULL,
	PRIMARY KEY (nome, legisl)
);`

// seedFirmatari popola la tabella firmatari dal seed embedded se è vuota.
func seedFirmatari(ctx context.Context, db *sql.DB) error {
	var n int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM firmatari`).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	stmt, err := tx.PrepareContext(ctx, `INSERT OR IGNORE INTO firmatari (nome, legisl, isis_expr) VALUES (?, ?, ?)`)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()
	for _, f := range ddlFirmatariSeed {
		if len(f.Legisl) == 0 {
			if _, err := stmt.ExecContext(ctx, f.Nome, "", f.ISIS); err != nil {
				tx.Rollback()
				return err
			}
		} else {
			for _, leg := range f.Legisl {
				if _, err := stmt.ExecContext(ctx, f.Nome, leg, f.ISIS); err != nil {
					tx.Rollback()
					return err
				}
			}
		}
	}
	return tx.Commit()
}

func newDdlFirmatariCmd(flags *rootFlags) *cobra.Command {
	var (
		flagLegisl string
		flagSearch string
		flagDB     string
	)

	cmd := &cobra.Command{
		Use:   "firmatari",
		Short: "Elenca i firmatari (deputati) per legislatura.",
		Long: `Elenca i firmatari dei disegni di legge con le legislature di appartenenza.

I dati sono precaricati nel database locale dal portale ARS.
Utile per scoprire i valori corretti da passare a --firmatario in 'ddl cerca'.`,
		Example: strings.Trim(`
  # Deputati della XVIII legislatura
  ars-sicilia-pp-cli ddl firmatari --legisl 18

  # Cerca per nome (tutte le legislature)
  ars-sicilia-pp-cli ddl firmatari --search "Cracolici"

  # In quali legislature ha operato un deputato?
  ars-sicilia-pp-cli ddl firmatari --search "Ardizzone" --json

  # Tutti i firmatari, output JSON
  ars-sicilia-pp-cli ddl firmatari --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if flagDB == "" {
				flagDB = defaultDBPath("ars-sicilia-pp-cli")
			}
			db, err := store.Open(flagDB)
			if err != nil {
				return fmt.Errorf("apertura database: %w", err)
			}
			defer db.Close()

			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			// Crea tabella e seed al primo uso.
			if _, err := db.DB().ExecContext(ctx, createFirmatariTable); err != nil {
				return fmt.Errorf("creazione tabella firmatari: %w", err)
			}
			if err := seedFirmatari(ctx, db.DB()); err != nil {
				return fmt.Errorf("seed firmatari: %w", err)
			}

			// Costruisce la query.
			where := []string{}
			qargs := []any{}
			if flagLegisl != "" {
				where = append(where, "legisl = ?")
				qargs = append(qargs, flagLegisl)
			}
			if flagSearch != "" {
				where = append(where, "nome LIKE ?")
				qargs = append(qargs, "%"+flagSearch+"%")
			}
			q := `SELECT nome, legisl, isis_expr FROM firmatari`
			if len(where) > 0 {
				q += " WHERE " + strings.Join(where, " AND ")
			}
			q += " ORDER BY nome, CAST(legisl AS INTEGER)"

			rows, err := db.DB().QueryContext(ctx, q, qargs...)
			if err != nil {
				return fmt.Errorf("query firmatari: %w", err)
			}
			defer rows.Close()

			type row struct {
				Nome   string `json:"nome"`
				Legisl string `json:"legisl,omitempty"`
				ISIS   string `json:"isis_expr,omitempty"`
			}
			var results []row
			for rows.Next() {
				var r row
				if err := rows.Scan(&r.Nome, &r.Legisl, &r.ISIS); err != nil {
					return err
				}
				results = append(results, r)
			}
			if err := rows.Err(); err != nil {
				return err
			}

			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "Nessun risultato.")
				return nil
			}

			if flags.asJSON || (!humanFriendly && !isTerminal(cmd.OutOrStdout())) {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(results)
			}

			// Output tabellare: raggruppa le legislature per nome.
			type grouped struct {
				nome   string
				legisl []string
			}
			var out []grouped
			cur := grouped{}
			for _, r := range results {
				if r.Nome != cur.nome {
					if cur.nome != "" {
						out = append(out, cur)
					}
					cur = grouped{nome: r.Nome}
				}
				if r.Legisl != "" {
					cur.legisl = append(cur.legisl, r.Legisl)
				}
			}
			if cur.nome != "" {
				out = append(out, cur)
			}

			for _, g := range out {
				if len(g.legisl) > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "%-40s  [%s]\n", g.nome, strings.Join(g.legisl, ", "))
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "%-40s  [tutte]\n", g.nome)
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&flagLegisl, "legisl", "", "Filtra per legislatura (es. 18).")
	cmd.Flags().StringVar(&flagSearch, "search", "", "Cerca per nome (parziale).")
	cmd.Flags().StringVar(&flagDB, "db", "", "Percorso database SQLite (default: ~/.local/share/ars-sicilia-pp-cli/data.db).")
	return cmd
}
