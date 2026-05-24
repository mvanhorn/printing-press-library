// Copyright 2026 justinwfu. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/coffee-goat/internal/store"
	"github.com/spf13/cobra"
)

// palateSignature is a normalised vector of descriptor weights, used
// both for export and import. Keys are lowercase, stripped descriptors
// (origin tokens + flavor tags); values are unit-vector weights.
type palateSignature struct {
	Name       string             `json:"name"`
	Source     string             `json:"source,omitempty"`
	Vector     map[string]float64 `json:"vector"`
	BrewsBased int                `json:"brews_based,omitempty"`
}

func newFriendPickCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "friend-pick",
		Short: "Recommend a bag for a friend whose palate profile has been imported",
		Example: `  coffee-goat-pp-cli friend-pick palate-export anne --out anne.json
  coffee-goat-pp-cli friend-pick palate-import anne.json
  coffee-goat-pp-cli friend-pick pick anne --from market --top 3`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newPalateExportCmd(flags))
	cmd.AddCommand(newPalateImportCmd(flags))
	cmd.AddCommand(newFriendPickRunCmd(flags))
	return cmd
}

func newPalateExportCmd(flags *rootFlags) *cobra.Command {
	var outPath string
	cmd := &cobra.Command{
		Use:     "palate-export <name>",
		Short:   "Export your palate signature derived from rated brews to a JSON file",
		Example: `  coffee-goat-pp-cli friend-pick palate-export anne --out anne.json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if strings.HasPrefix(args[0], "__printing_press") {
				return usageErr(fmt.Errorf("invalid palate name %q (reserved sentinel)", args[0]))
			}
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("coffee-goat-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()
			sig, err := derivePalateSignature(db, args[0])
			if err != nil {
				return err
			}
			b, err := json.MarshalIndent(sig, "", "  ")
			if err != nil {
				return err
			}
			if outPath == "" {
				outPath = args[0] + ".palate.json"
			}
			if err := os.WriteFile(outPath, b, 0o644); err != nil {
				return err
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"status": "exported", "path": outPath}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "exported palate %q to %s\n", args[0], outPath)
			return nil
		},
	}
	cmd.Flags().StringVar(&outPath, "out", "", "Output file path (default: <name>.palate.json)")
	return cmd
}

func newPalateImportCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "palate-import <path>",
		Short:   "Import a friend's palate signature JSON into the local store",
		Example: `  coffee-goat-pp-cli friend-pick palate-import anne.json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			data, err := os.ReadFile(args[0])
			if err != nil {
				return err
			}
			var sig palateSignature
			if err := json.Unmarshal(data, &sig); err != nil {
				return err
			}
			if sig.Name == "" {
				return fmt.Errorf("palate-import: signature is missing 'name' field")
			}
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("coffee-goat-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()
			sigJSON, _ := json.Marshal(sig)
			_, err = db.DB().Exec(
				`INSERT INTO palate_profiles (name, signature_json, source) VALUES (?, ?, ?)
				 ON CONFLICT(name) DO UPDATE SET signature_json=excluded.signature_json, source=excluded.source, imported_at=CURRENT_TIMESTAMP`,
				sig.Name, string(sigJSON), sig.Source,
			)
			if err != nil {
				return err
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"status": "imported", "name": sig.Name}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "imported palate %q\n", sig.Name)
			return nil
		},
	}
}

type friendPickResult struct {
	Roaster    string  `json:"roaster"`
	Handle     string  `json:"handle"`
	Title      string  `json:"title"`
	Origin     string  `json:"origin,omitempty"`
	Process    string  `json:"process,omitempty"`
	Similarity float64 `json:"similarity_score"`
	Rationale  string  `json:"rationale"`
}

func newFriendPickRunCmd(flags *rootFlags) *cobra.Command {
	var top int
	var from string
	cmd := &cobra.Command{
		Use:         "pick <name>",
		Short:       "Rank current shelf or market beans for a friend's palate profile",
		Example:     `  coffee-goat-pp-cli friend-pick pick anne --from market --top 3 --agent`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("coffee-goat-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()
			var sigJSON string
			err = db.DB().QueryRow(`SELECT signature_json FROM palate_profiles WHERE name=?`, args[0]).Scan(&sigJSON)
			if err != nil {
				return notFoundErr(fmt.Errorf("friend-pick: no palate profile named %q (import one with 'friend-pick palate-import')", args[0]))
			}
			var sig palateSignature
			if err := json.Unmarshal([]byte(sigJSON), &sig); err != nil {
				return err
			}
			results, err := rankForPalate(db, sig, from, top)
			if err != nil {
				return err
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"friend":     sig.Name,
					"rationale":  fmt.Sprintf("ranked against friend %q's signature: %d dims, %d brews-based", sig.Name, len(sig.Vector), sig.BrewsBased),
					"candidates": results,
				}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "friend %q palate top picks:\n", sig.Name)
			for _, r := range results {
				fmt.Fprintf(cmd.OutOrStdout(), "  %.2f  %s / %s (%s)\n", r.Similarity, r.Roaster, r.Title, r.Origin)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&top, "top", 5, "Number of picks to return")
	cmd.Flags().StringVar(&from, "from", "market", "Pool to rank from: market (all roaster_products) or shelf (your beans)")
	return cmd
}

func derivePalateSignature(db *store.Store, name string) (palateSignature, error) {
	rows, err := db.DB().Query(
		`SELECT COALESCE(b.rating, 0),
		        COALESCE(rp.origin,''), COALESCE(rp.process,''), COALESCE(rp.varietal,''),
		        COALESCE(rp.tags_json,''), COALESCE(rp.body_text,'')
		 FROM brews b
		 LEFT JOIN beans bn ON b.bean_id = bn.id
		 LEFT JOIN roaster_products rp ON bn.roaster_slug = rp.roaster_slug AND bn.product_slug = rp.handle
		 WHERE b.rating > 0`,
	)
	if err != nil {
		return palateSignature{}, err
	}
	defer rows.Close()
	vec := map[string]float64{}
	count := 0
	for rows.Next() {
		var rating int
		var origin, process, varietal, tagsJSON, body string
		if err := rows.Scan(&rating, &origin, &process, &varietal, &tagsJSON, &body); err != nil {
			return palateSignature{}, err
		}
		count++
		w := float64(rating - 5) // -4..+5 weight
		for _, tok := range splitDescriptors(origin + " " + process + " " + varietal + " " + tagsJSON + " " + body) {
			vec[tok] += w
		}
	}
	if err := rows.Err(); err != nil {
		return palateSignature{}, fmt.Errorf("iterate brews rows: %w", err)
	}
	// Normalise.
	mag := 0.0
	for _, v := range vec {
		mag += v * v
	}
	mag = math.Sqrt(mag)
	if mag > 0 {
		for k := range vec {
			vec[k] /= mag
		}
	}
	return palateSignature{Name: name, Source: "self-export", Vector: vec, BrewsBased: count}, nil
}

func rankForPalate(db *store.Store, sig palateSignature, from string, top int) ([]friendPickResult, error) {
	if top <= 0 {
		top = 5
	}
	rows, err := db.DB().Query(
		`SELECT roaster_slug, handle, COALESCE(title,''), COALESCE(origin,''), COALESCE(process,''), COALESCE(varietal,''), COALESCE(tags_json,''), COALESCE(body_text,'')
		 FROM roaster_products`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []friendPickResult
	for rows.Next() {
		var r, h, title, origin, process, varietal, tagsJSON, body string
		if err := rows.Scan(&r, &h, &title, &origin, &process, &varietal, &tagsJSON, &body); err != nil {
			return nil, err
		}
		toks := splitDescriptors(origin + " " + process + " " + varietal + " " + tagsJSON + " " + body)
		score := 0.0
		for _, t := range toks {
			score += sig.Vector[t]
		}
		out = append(out, friendPickResult{
			Roaster: r, Handle: h, Title: title, Origin: origin, Process: process,
			Similarity: math.Round(score*100) / 100,
			Rationale:  fmt.Sprintf("matched %d descriptor tokens to %s's palate", overlap(toks, sig.Vector), sig.Name),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate roaster_products rows: %w", err)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Similarity > out[j].Similarity })
	if top < len(out) {
		out = out[:top]
	}
	return out, nil
}

func splitDescriptors(s string) []string {
	cleaner := strings.NewReplacer(
		`[`, " ", `]`, " ", `"`, " ", `,`, " ", `.`, " ",
		`(`, " ", `)`, " ", `:`, " ", `;`, " ", `\n`, " ",
	)
	cleaned := cleaner.Replace(strings.ToLower(s))
	parts := strings.Fields(cleaned)
	uniq := map[string]bool{}
	var out []string
	for _, p := range parts {
		if len(p) < 3 {
			continue
		}
		if uniq[p] {
			continue
		}
		uniq[p] = true
		out = append(out, p)
	}
	return out
}

func overlap(toks []string, vec map[string]float64) int {
	n := 0
	for _, t := range toks {
		if _, ok := vec[t]; ok {
			n++
		}
	}
	return n
}
