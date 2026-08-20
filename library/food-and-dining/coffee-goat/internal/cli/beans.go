// Copyright 2026 Justin Fu and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/coffee-goat/internal/store"
	"github.com/spf13/cobra"
)

// beanRow is the JSON / table shape for cellar entries.
type beanRow struct {
	ID             int64  `json:"id"`
	RoasterSlug    string `json:"roaster_slug,omitempty"`
	ProductSlug    string `json:"product_slug,omitempty"`
	ProductTitle   string `json:"product_title,omitempty"`
	RoastDate      string `json:"roast_date,omitempty"`
	PurchaseDate   string `json:"purchase_date,omitempty"`
	PricePaidCents int    `json:"price_paid_cents,omitempty"`
	CurrentMassG   int    `json:"current_mass_g,omitempty"`
	Notes          string `json:"notes,omitempty"`
	AddedAt        string `json:"added_at,omitempty"`
}

func newBeansCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cellar",
		Short: "Local cellar (beans on the shelf). Subcommands: add, list, show, update, remove",
		Example: `  coffee-goat-pp-cli brews cellar add --roaster onyx --product geisha-honey --roast-date 2026-05-10 --mass-g 250
  coffee-goat-pp-cli brews cellar list --agent`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newBeansAddCmd(flags))
	cmd.AddCommand(newBeansListCmd(flags))
	cmd.AddCommand(newBeansShowCmd(flags))
	cmd.AddCommand(newBeansUpdateCmd(flags))
	cmd.AddCommand(newBeansRemoveCmd(flags))
	return cmd
}

func newBeansAddCmd(flags *rootFlags) *cobra.Command {
	var (
		roaster      string
		product      string
		roastDate    string
		purchaseDate string
		pricePaid    int
		massG        int
		notes        string
	)
	cmd := &cobra.Command{
		Use:     "add",
		Short:   "Add a bag to the local cellar. --roaster + --product identify the matching roaster_products row (optional but enables joins)",
		Example: `  coffee-goat-pp-cli brews cellar add --roaster sey --product banko-gotiti --roast-date 2026-05-10 --mass-g 200`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("coffee-goat-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()
			res, err := db.DB().Exec(
				`INSERT INTO beans (roaster_slug, product_slug, roast_date, purchase_date, price_paid_cents, current_mass_g, notes, added_at)
				 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
				nullableString(roaster), nullableString(product),
				nullableString(roastDate), nullableString(purchaseDate),
				nullableInt(pricePaid), nullableInt(massG),
				nullableString(notes), time.Now().UTC().Format(time.RFC3339),
			)
			if err != nil {
				return fmt.Errorf("insert bean: %w", err)
			}
			id, _ := res.LastInsertId()
			row := beanRow{
				ID: id, RoasterSlug: roaster, ProductSlug: product,
				RoastDate: roastDate, PurchaseDate: purchaseDate,
				PricePaidCents: pricePaid, CurrentMassG: massG, Notes: notes,
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), row, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "added bean #%d (%s/%s)\n", id, roaster, product)
			return nil
		},
	}
	cmd.Flags().StringVar(&roaster, "roaster", "", "Roaster slug, from the synced corpus, e.g. onyx, sey, or partners")
	cmd.Flags().StringVar(&product, "product", "", "Product handle, from roaster_products, the per-roaster URL slug")
	cmd.Flags().StringVar(&roastDate, "roast-date", "", "Roast date in YYYY-MM-DD, used for freshness windows, peak ranges, drift")
	cmd.Flags().StringVar(&purchaseDate, "purchase-date", "", "Purchase date in YYYY-MM-DD, used for budget rollups, refill rate, depletion")
	cmd.Flags().IntVar(&pricePaid, "price-cents", 0, "Price paid at purchase, in cents, used by budget and cost-per-cup attribution math")
	cmd.Flags().IntVar(&massG, "mass-g", 0, "Current bag mass, grams, used by shelf depletion and refill-plan signals")
	cmd.Flags().StringVar(&notes, "notes", "", "Free-text tasting notes or buying-context notes attached to this cellar entry")
	_ = cmd.MarkFlagRequired("roaster")
	_ = cmd.MarkFlagRequired("product")
	return cmd
}

func newBeansListCmd(flags *rootFlags) *cobra.Command {
	var roaster string
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List bags currently in the cellar. Columns: id, roaster/product, roast_date, mass_g, added_at",
		Example:     `  coffee-goat-pp-cli brews cellar list --agent`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("coffee-goat-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()
			rows, err := queryBeans(db, 0, roaster)
			if err != nil {
				return err
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "cellar empty (add a bag via 'brews cellar add')")
				return nil
			}
			for _, r := range rows {
				label := r.RoasterSlug + "/" + r.ProductSlug
				if r.ProductTitle != "" {
					label += " (" + r.ProductTitle + ")"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  #%d  %s  roasted=%s  mass=%dg  %s\n",
					r.ID, label, r.RoastDate, r.CurrentMassG, r.AddedAt)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&roaster, "roaster", "", "Restrict to one roaster slug")
	return cmd
}

func newBeansShowCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "show <id>",
		Short:       "Show full detail for one bag in the cellar",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return usageErr(fmt.Errorf("bean id must be an integer (got %q)", args[0]))
			}
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("coffee-goat-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()
			rows, err := queryBeans(db, id, "")
			if err != nil {
				return err
			}
			if len(rows) == 0 {
				return notFoundErr(fmt.Errorf("bean #%d not found", id))
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), rows[0], flags)
			}
			r := rows[0]
			fmt.Fprintf(cmd.OutOrStdout(), "bean #%d\n  roaster/product: %s/%s\n  title: %s\n  roast_date: %s\n  purchase_date: %s\n  price_paid: %d cents\n  mass: %dg\n  notes: %s\n  added_at: %s\n",
				r.ID, r.RoasterSlug, r.ProductSlug, r.ProductTitle, r.RoastDate, r.PurchaseDate, r.PricePaidCents, r.CurrentMassG, r.Notes, r.AddedAt)
			return nil
		},
	}
	return cmd
}

func newBeansUpdateCmd(flags *rootFlags) *cobra.Command {
	var massG int
	var roastDate, notes string
	cmd := &cobra.Command{
		Use:     "update <id>",
		Short:   "Update mutable fields on a cellar bag (mass after a brew, late-arrived roast date, notes)",
		Example: `  coffee-goat-pp-cli brews cellar update 7 --mass-g 175`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return usageErr(fmt.Errorf("bean id must be an integer (got %q)", args[0]))
			}
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("coffee-goat-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()

			sets := []string{}
			argv := []any{}
			if cmd.Flags().Changed("mass-g") {
				sets = append(sets, "current_mass_g=?")
				argv = append(argv, massG)
			}
			if cmd.Flags().Changed("roast-date") {
				sets = append(sets, "roast_date=?")
				argv = append(argv, roastDate)
			}
			if cmd.Flags().Changed("notes") {
				sets = append(sets, "notes=?")
				argv = append(argv, notes)
			}
			if len(sets) == 0 {
				return usageErr(fmt.Errorf("brews cellar update requires at least one of --mass-g, --roast-date, --notes"))
			}
			argv = append(argv, id)
			res, err := db.DB().Exec(`UPDATE beans SET `+strings.Join(sets, ", ")+` WHERE id=?`, argv...)
			if err != nil {
				return fmt.Errorf("update bean: %w", err)
			}
			n, _ := res.RowsAffected()
			if n == 0 {
				return notFoundErr(fmt.Errorf("bean #%d not found", id))
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"updated": id}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "updated bean #%d\n", id)
			return nil
		},
	}
	cmd.Flags().IntVar(&massG, "mass-g", 0, "New current mass in grams")
	cmd.Flags().StringVar(&roastDate, "roast-date", "", "Set roast date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&notes, "notes", "", "Overwrite notes")
	return cmd
}

func newBeansRemoveCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "remove <id>",
		Short:   "Remove a bag from the cellar (cascades to its brews)",
		Example: `  coffee-goat-pp-cli brews cellar remove 7 --yes`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return usageErr(fmt.Errorf("bean id must be an integer (got %q)", args[0]))
			}
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("coffee-goat-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()
			res, err := db.DB().Exec(`DELETE FROM beans WHERE id=?`, id)
			if err != nil {
				return fmt.Errorf("delete bean: %w", err)
			}
			n, _ := res.RowsAffected()
			if n == 0 {
				return notFoundErr(fmt.Errorf("bean #%d not found", id))
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"removed": id}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "removed bean #%d\n", id)
			return nil
		},
	}
	return cmd
}

// queryBeans returns cellar rows joined to roaster_products.title when
// the bean's (roaster_slug, product_slug) matches a synced row. Pass
// id=0 + roaster="" for "all rows".
func queryBeans(db *store.Store, id int64, roaster string) ([]beanRow, error) {
	q := `SELECT b.id, COALESCE(b.roaster_slug,''), COALESCE(b.product_slug,''),
	             COALESCE(rp.title,''),
	             COALESCE(b.roast_date,''), COALESCE(b.purchase_date,''),
	             COALESCE(b.price_paid_cents,0), COALESCE(b.current_mass_g,0),
	             COALESCE(b.notes,''), COALESCE(b.added_at,'')
	      FROM beans b
	      LEFT JOIN roaster_products rp ON b.roaster_slug = rp.roaster_slug AND b.product_slug = rp.handle
	      WHERE 1=1`
	args := []any{}
	if id != 0 {
		q += ` AND b.id = ?`
		args = append(args, id)
	}
	if roaster != "" {
		q += ` AND b.roaster_slug = ?`
		args = append(args, roaster)
	}
	q += ` ORDER BY b.added_at DESC, b.id DESC`
	rows, err := db.DB().Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []beanRow
	for rows.Next() {
		var r beanRow
		if err := rows.Scan(
			&r.ID, &r.RoasterSlug, &r.ProductSlug,
			&r.ProductTitle,
			&r.RoastDate, &r.PurchaseDate,
			&r.PricePaidCents, &r.CurrentMassG,
			&r.Notes, &r.AddedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	return out, nil
}
