// Copyright 2026 h179922. Licensed under Apache-2.0. See LICENSE.
// Novel command: project tracker — group products into named renovation/design projects with budgets.

package cli

import (
	"database/sql"
	"fmt"

	"github.com/spf13/cobra"
)

func newProjectCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project",
		Short: "Manage home renovation/design projects with product lists and budgets",
		Long:  "Create named projects (e.g. \"Kitchen Reno\"), add products from any source, and track running budgets across all sources.",
		RunE:  parentNoSubcommandRunE(flags),
	}

	cmd.AddCommand(newProjectCreateCmd(flags))
	cmd.AddCommand(newProjectListCmd(flags))
	cmd.AddCommand(newProjectShowCmd(flags))
	cmd.AddCommand(newProjectAddCmd(flags))
	cmd.AddCommand(newProjectRemoveCmd(flags))
	cmd.AddCommand(newProjectBudgetCmd(flags))
	cmd.AddCommand(newProjectDeleteCmd(flags))

	return cmd
}

func newProjectCreateCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "create <name>",
		Short:   "Create a named project",
		Example: "  reno-goat-pp-cli project create \"Kitchen Reno\"",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			name := args[0]

			db, err := openNovelDB()
			if err != nil {
				return err
			}
			defer db.Close()

			_, err = db.Exec(`INSERT INTO projects (name) VALUES (?)`, name)
			if err != nil {
				// Check for unique constraint violation
				return fmt.Errorf("creating project: %w (project %q may already exist)", err, name)
			}

			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"status": "created",
				"name":   name,
			}, flags)
		},
	}
}

func newProjectListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all projects with item counts and budget totals",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			db, err := openNovelDB()
			if err != nil {
				return err
			}
			defer db.Close()

			rows, err := db.Query(`
				SELECT p.id, p.name, p.created_at,
				       COUNT(pi.id) AS item_count,
				       COALESCE(SUM(pi.price * pi.quantity), 0) AS total_budget
				FROM projects p
				LEFT JOIN project_items pi ON pi.project_id = p.id
				GROUP BY p.id
				ORDER BY p.created_at DESC`)
			if err != nil {
				return fmt.Errorf("listing projects: %w", err)
			}
			defer rows.Close()

			var projects []map[string]any
			for rows.Next() {
				var (
					id        int64
					name      string
					createdAt string
					itemCount int
					budget    float64
				)
				if err := rows.Scan(&id, &name, &createdAt, &itemCount, &budget); err != nil {
					return fmt.Errorf("scanning project: %w", err)
				}
				projects = append(projects, map[string]any{
					"id":         id,
					"name":       name,
					"created_at": createdAt,
					"item_count": itemCount,
					"budget":     budget,
				})
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating projects: %w", err)
			}

			if len(projects) == 0 {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"projects": []any{},
					"count":    0,
				}, flags)
			}

			return printJSONFiltered(cmd.OutOrStdout(), projects, flags)
		},
	}
}

func newProjectShowCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "show <name>",
		Short:   "Show a project's products with prices and budget",
		Example: "  reno-goat-pp-cli project show \"Kitchen Reno\"",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			name := args[0]

			db, err := openNovelDB()
			if err != nil {
				return err
			}
			defer db.Close()

			var projectID int64
			var createdAt string
			err = db.QueryRow(`SELECT id, created_at FROM projects WHERE name = ?`, name).Scan(&projectID, &createdAt)
			if err == sql.ErrNoRows {
				return notFoundErr(fmt.Errorf("project %q not found", name))
			}
			if err != nil {
				return fmt.Errorf("looking up project: %w", err)
			}

			rows, err := db.Query(`
				SELECT product_url, source, title, price, quantity, added_at
				FROM project_items WHERE project_id = ? ORDER BY added_at DESC`, projectID)
			if err != nil {
				return fmt.Errorf("listing project items: %w", err)
			}
			defer rows.Close()

			var items []map[string]any
			var totalBudget float64
			for rows.Next() {
				var (
					productURL, source string
					title              sql.NullString
					price              sql.NullFloat64
					quantity           int
					addedAt            string
				)
				if err := rows.Scan(&productURL, &source, &title, &price, &quantity, &addedAt); err != nil {
					return fmt.Errorf("scanning item: %w", err)
				}
				item := map[string]any{
					"product_url": productURL,
					"source":      source,
					"quantity":    quantity,
					"added_at":    addedAt,
				}
				if title.Valid {
					item["title"] = title.String
				}
				if price.Valid {
					item["price"] = price.Float64
					item["line_total"] = price.Float64 * float64(quantity)
					totalBudget += price.Float64 * float64(quantity)
				}
				items = append(items, item)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating items: %w", err)
			}

			result := map[string]any{
				"name":       name,
				"created_at": createdAt,
				"items":      items,
				"item_count": len(items),
				"budget":     totalBudget,
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
}

func newProjectAddCmd(flags *rootFlags) *cobra.Command {
	var quantity int
	var price float64
	var title string

	cmd := &cobra.Command{
		Use:     "add <name> <product-url>",
		Short:   "Add a product to a project",
		Example: "  reno-goat-pp-cli project add \"Kitchen Reno\" https://www.westelm.com/products/faucet --quantity 2 --price 299.99",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			name := args[0]
			productURL := args[1]
			source := inferSource(productURL)

			db, err := openNovelDB()
			if err != nil {
				return err
			}
			defer db.Close()

			var projectID int64
			err = db.QueryRow(`SELECT id FROM projects WHERE name = ?`, name).Scan(&projectID)
			if err == sql.ErrNoRows {
				return notFoundErr(fmt.Errorf("project %q not found; create it first with: reno-goat-pp-cli project create %q", name, name))
			}
			if err != nil {
				return fmt.Errorf("looking up project: %w", err)
			}

			var titlePtr *string
			if title != "" {
				titlePtr = &title
			}
			var pricePtr *float64
			if price > 0 {
				pricePtr = &price
			}
			var quantityPtr *int
			if cmd.Flags().Changed("quantity") {
				quantityPtr = &quantity
			}

			_, err = db.Exec(
				`INSERT INTO project_items (project_id, product_url, source, title, price, quantity)
				 VALUES (?, ?, ?, ?, ?, COALESCE(?, 1))
				 ON CONFLICT(project_id, product_url) DO UPDATE SET
				   quantity = COALESCE(excluded.quantity, project_items.quantity),
				   price = COALESCE(excluded.price, project_items.price),
				   title = COALESCE(excluded.title, project_items.title)`,
				projectID, productURL, source, titlePtr, pricePtr, quantityPtr,
			)
			if err != nil {
				return fmt.Errorf("adding item to project: %w", err)
			}

			result := map[string]any{
				"status":      "added",
				"project":     name,
				"product_url": productURL,
				"source":      source,
				"quantity":    quantity,
			}
			if price > 0 {
				result["price"] = price
			}
			if title != "" {
				result["title"] = title
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().IntVar(&quantity, "quantity", 1, "Quantity of this product")
	cmd.Flags().Float64Var(&price, "price", 0, "Price per unit")
	cmd.Flags().StringVar(&title, "title", "", "Product title/description")
	return cmd
}

func newProjectRemoveCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "remove <name> <product-url>",
		Short:   "Remove a product from a project",
		Example: "  reno-goat-pp-cli project remove \"Kitchen Reno\" https://www.westelm.com/products/faucet",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			name := args[0]
			productURL := args[1]

			db, err := openNovelDB()
			if err != nil {
				return err
			}
			defer db.Close()

			var projectID int64
			err = db.QueryRow(`SELECT id FROM projects WHERE name = ?`, name).Scan(&projectID)
			if err == sql.ErrNoRows {
				return notFoundErr(fmt.Errorf("project %q not found", name))
			}
			if err != nil {
				return fmt.Errorf("looking up project: %w", err)
			}

			res, err := db.Exec(`DELETE FROM project_items WHERE project_id = ? AND product_url = ?`, projectID, productURL)
			if err != nil {
				return fmt.Errorf("removing item: %w", err)
			}
			affected, _ := res.RowsAffected()
			if affected == 0 {
				return notFoundErr(fmt.Errorf("product %q not found in project %q", productURL, name))
			}

			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"status":      "removed",
				"project":     name,
				"product_url": productURL,
			}, flags)
		},
	}
}

func newProjectBudgetCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "budget <name>",
		Short:   "Show running budget total across all sources",
		Example: "  reno-goat-pp-cli project budget \"Kitchen Reno\"",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			name := args[0]

			db, err := openNovelDB()
			if err != nil {
				return err
			}
			defer db.Close()

			var projectID int64
			err = db.QueryRow(`SELECT id FROM projects WHERE name = ?`, name).Scan(&projectID)
			if err == sql.ErrNoRows {
				return notFoundErr(fmt.Errorf("project %q not found", name))
			}
			if err != nil {
				return fmt.Errorf("looking up project: %w", err)
			}

			rows, err := db.Query(`
				SELECT source,
				       COUNT(*) AS item_count,
				       SUM(quantity) AS total_quantity,
				       COALESCE(SUM(price * quantity), 0) AS subtotal
				FROM project_items
				WHERE project_id = ?
				GROUP BY source
				ORDER BY subtotal DESC`, projectID)
			if err != nil {
				return fmt.Errorf("calculating budget: %w", err)
			}
			defer rows.Close()

			var bySource []map[string]any
			var grandTotal float64
			for rows.Next() {
				var (
					source        string
					itemCount     int
					totalQuantity int
					subtotal      float64
				)
				if err := rows.Scan(&source, &itemCount, &totalQuantity, &subtotal); err != nil {
					return fmt.Errorf("scanning budget row: %w", err)
				}
				bySource = append(bySource, map[string]any{
					"source":         source,
					"item_count":     itemCount,
					"total_quantity": totalQuantity,
					"subtotal":       subtotal,
				})
				grandTotal += subtotal
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating budget: %w", err)
			}

			result := map[string]any{
				"project":     name,
				"by_source":   bySource,
				"grand_total": grandTotal,
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
}

func newProjectDeleteCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "delete <name>",
		Short:   "Delete a project and all its items",
		Example: "  reno-goat-pp-cli project delete \"Kitchen Reno\"",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			name := args[0]

			db, err := openNovelDB()
			if err != nil {
				return err
			}
			defer db.Close()

			res, err := db.Exec(`DELETE FROM projects WHERE name = ?`, name)
			if err != nil {
				return fmt.Errorf("deleting project: %w", err)
			}
			affected, _ := res.RowsAffected()
			if affected == 0 {
				return notFoundErr(fmt.Errorf("project %q not found", name))
			}

			// project_items cascade-deleted via ON DELETE CASCADE

			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"status": "deleted",
				"name":   name,
			}, flags)
		},
	}
}
