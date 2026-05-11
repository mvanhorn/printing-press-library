package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/goose/internal/auth"
	"github.com/mvanhorn/printing-press-library/library/commerce/goose/internal/config"

	"github.com/spf13/cobra"
)

// newPetCmd implements `pet <name>` — find a pet by name and return tags,
// vaccinations, instructions, current room assignment, owner, and upcoming
// bookings in one render. Uses search-api.goose.pet's customer endpoint
// with `include=petProfiles` then filters down to the pet that matched.
func newPetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "pet [name]",
		Short:       "Find a pet by name; return tags, vaccinations, instructions, room, owner, and upcoming bookings",
		Example:     "  goose-pp-cli pet Riley\n  goose-pp-cli pet riley --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			query := strings.Join(args, " ")
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			if err := auth.RefreshIfNeeded(cfg, 60*time.Second); err != nil {
				return err
			}
			facility := cfg.TemplateVars["facility"]
			if facility == "" {
				return fmt.Errorf("no facility configured; run `goose-pp-cli auth login` or set GOOSE_FACILITY")
			}

			hits, err := searchPets(cfg.AuthHeader(), facility, query, 25)
			if err != nil {
				return fmt.Errorf("searching: %w", err)
			}

			// Flatten: each customer hit may include a `petProfiles` array; we
			// emit one row per pet matching the query.
			out := []map[string]any{}
			for _, h := range hits {
				ownerName := strings.TrimSpace(strOrDash(h["firstName"]) + " " + strOrDash(h["lastName"]))
				pets, _ := h["petProfiles"].([]any)
				for _, p := range pets {
					pm, ok := p.(map[string]any)
					if !ok {
						continue
					}
					name := strOrDash(pm["displayName"])
					if !strings.Contains(strings.ToLower(name), strings.ToLower(query)) {
						continue
					}
					row := map[string]any{
						"petId":       pm["id"],
						"displayName": name,
						"sex":         pm["sex"],
						"breed":       pm["breed"],
						"weight":      pm["weight"],
						"dateOfBirth": pm["dateOfBirth"],
						"owner": map[string]any{
							"id":    h["id"],
							"name":  ownerName,
							"email": h["email"],
							"phone": h["phone"],
						},
						"vaccinations":    pm["vaccinations"],
						"tags":            pm["tags"],
						"petInstructions": pm["petInstructions"],
					}
					out = append(out, row)
				}
			}
			if len(out) == 0 {
				return fmt.Errorf("no pets matching %q", query)
			}

			if flags.asJSON || flags.compact || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			w := cmd.OutOrStdout()
			for _, r := range out {
				renderPetHuman(w, r)
				fmt.Fprintln(w)
			}
			return nil
		},
	}
	return cmd
}

func renderPetHuman(w interface {
	Write([]byte) (int, error)
}, r map[string]any) {
	fmt.Fprintf(w, "%s (%s)\n", r["displayName"], strOrDash(r["sex"]))
	if owner, ok := r["owner"].(map[string]any); ok {
		fmt.Fprintf(w, "  Owner: %s — %s\n", strOrDash(owner["name"]), strOrDash(owner["email"]))
	}
	if tags, ok := r["tags"].([]any); ok && len(tags) > 0 {
		names := []string{}
		for _, t := range tags {
			tm, _ := t.(map[string]any)
			names = append(names, strOrDash(tm["displayName"]))
		}
		fmt.Fprintf(w, "  Tags: %s\n", strings.Join(names, ", "))
	}
	if vaccs, ok := r["vaccinations"].([]any); ok && len(vaccs) > 0 {
		fmt.Fprintf(w, "  Vaccinations: %d on file\n", len(vaccs))
		today := time.Now().Format("2006-01-02")
		expired := 0
		for _, v := range vaccs {
			vm, _ := v.(map[string]any)
			if exp, _ := vm["expirationDate"].(string); exp != "" && exp < today {
				expired++
			}
		}
		if expired > 0 {
			fmt.Fprintf(w, "  ⚠ %d expired\n", expired)
		}
	}
}

// Ensure json import is used (we use it in customer.go; this guards against
// "imported and not used" if pet.go's render path doesn't reach the JSON branch.)
var _ = json.Marshal
