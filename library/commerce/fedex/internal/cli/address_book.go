// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/store"

	"github.com/spf13/cobra"
)

func newAddressCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "address",
		Short: "Local address book + cached address validation (singular; the generated 'addresses' command remains)",
	}
	cmd.AddCommand(newAddressSaveCmd(flags))
	cmd.AddCommand(newAddressListCmd(flags))
	cmd.AddCommand(newAddressDeleteCmd(flags))
	cmd.AddCommand(newAddressValidateCachedCmd(flags))
	return cmd
}

func newAddressSaveCmd(flags *rootFlags) *cobra.Command {
	var (
		contactName string
		company     string
		phone       string
		email       string
		street      string
		street2     string
		city        string
		state       string
		zip         string
		country     string
		residential bool
		notes       string
	)
	cmd := &cobra.Command{
		Use:   "save [name]",
		Short: "Save a recipient to the local address book",
		Example: strings.Trim(`
  fedex-pp-cli address save acme --contact-name "ACME Corp" --street "1 Anvil Way" --city Burbank --state CA --zip 91505
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			name := strings.TrimSpace(args[0])
			if street == "" || city == "" || zip == "" {
				return usageErr(fmt.Errorf("--street, --city, and --zip are required"))
			}
			if dryRunOK(flags) {
				return nil
			}
			st, err := store.Open("")
			if err != nil {
				return err
			}
			defer st.Close()
			ctx := context.Background()
			if err := st.UpsertAddress(ctx, store.Address{
				Name:          name,
				ContactName:   contactName,
				Company:       company,
				Phone:         phone,
				Email:         email,
				Street:        street,
				Street2:       street2,
				City:          city,
				State:         state,
				Postal:        zip,
				Country:       country,
				IsResidential: residential,
				Notes:         notes,
			}); err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"saved": name}, flags)
		},
	}
	cmd.Flags().StringVar(&contactName, "contact-name", "", "Recipient contact name")
	cmd.Flags().StringVar(&company, "company", "", "Recipient company")
	cmd.Flags().StringVar(&phone, "phone", "", "Recipient phone")
	cmd.Flags().StringVar(&email, "email", "", "Recipient email")
	cmd.Flags().StringVar(&street, "street", "", "Street line 1")
	cmd.Flags().StringVar(&street2, "street2", "", "Street line 2")
	cmd.Flags().StringVar(&city, "city", "", "City")
	cmd.Flags().StringVar(&state, "state", "", "State or province code")
	cmd.Flags().StringVar(&zip, "zip", "", "Postal code")
	cmd.Flags().StringVar(&country, "country", "US", "ISO country code")
	cmd.Flags().BoolVar(&residential, "residential", false, "Mark as residential address")
	cmd.Flags().StringVar(&notes, "notes", "", "Free-form notes")
	return cmd
}

func newAddressListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List saved address book entries",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			st, err := store.Open("")
			if err != nil {
				return err
			}
			defer st.Close()
			ctx := context.Background()
			items, err := st.ListAddresses(ctx)
			if err != nil {
				return err
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				headers := []string{"NAME", "CONTACT", "COMPANY", "CITY", "STATE", "POSTAL", "COUNTRY"}
				rows := make([][]string, 0, len(items))
				for _, a := range items {
					rows = append(rows, []string{a.Name, a.ContactName, a.Company, a.City, a.State, a.Postal, a.Country})
				}
				return flags.printTable(cmd, headers, rows)
			}
			return printJSONFiltered(cmd.OutOrStdout(), items, flags)
		},
	}
	return cmd
}

func newAddressDeleteCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete [name]",
		Short: "Delete an address book entry by name",
		Example: `  # Remove a saved recipient by its short name
  fedex-pp-cli address delete acme-warehouse

  # Use --json to get a structured deletion confirmation
  fedex-pp-cli address delete acme-warehouse --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			name := strings.TrimSpace(args[0])
			if dryRunOK(flags) {
				return nil
			}
			st, err := store.Open("")
			if err != nil {
				return err
			}
			defer st.Close()
			ctx := context.Background()
			ok, err := st.DeleteAddress(ctx, name)
			if err != nil {
				return err
			}
			if !ok {
				return notFoundErr(fmt.Errorf("address %q not found", name))
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"deleted": name}, flags)
		},
	}
	return cmd
}

func newAddressValidateCachedCmd(flags *rootFlags) *cobra.Command {
	var (
		street   string
		city     string
		state    string
		zip      string
		country  string
		useCache bool
	)
	cmd := &cobra.Command{
		Use:         "validate",
		Short:       "Validate an address with a SHA-256 keyed local cache (skips API on repeat lookups)",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if street == "" || city == "" || zip == "" {
				return cmd.Help()
			}
			if country == "" {
				country = "US"
			}
			if dryRunOK(flags) {
				return nil
			}

			key := addressCacheKey(street, city, state, zip, country)
			st, _ := store.Open("")
			if st != nil {
				defer st.Close()
			}
			ctx := context.Background()

			if useCache && st != nil {
				if av, _ := st.GetAddressValidationByKey(ctx, key); av != nil {
					return printJSONFiltered(cmd.OutOrStdout(), addressValidationToJSON(av, true), flags)
				}
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			body := map[string]any{
				"addressesToValidate": []any{
					map[string]any{
						"address": map[string]any{
							"streetLines":         []string{street},
							"city":                city,
							"stateOrProvinceCode": state,
							"postalCode":          zip,
							"countryCode":         country,
						},
					},
				},
			}
			data, _, err := c.Post("/address/v1/addresses/resolve", body)
			if err != nil {
				return classifyAPIError(err)
			}
			classification, rs, rc, rst, rp := parseAddressResolve(data)
			if st != nil {
				_ = st.InsertAddressValidation(ctx, store.AddressValidationCache{
					CacheKey:       key,
					Street:         street,
					City:           city,
					State:          state,
					Postal:         zip,
					Country:        country,
					Classification: classification,
					ResolvedStreet: rs,
					ResolvedCity:   rc,
					ResolvedState:  rst,
					ResolvedPostal: rp,
					RawResponse:    string(data),
				})
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"cache_hit":      false,
				"classification": classification,
				"resolved": map[string]any{
					"street": rs,
					"city":   rc,
					"state":  rst,
					"postal": rp,
				},
			}, flags)
		},
	}
	cmd.Flags().StringVar(&street, "street", "", "Street line")
	cmd.Flags().StringVar(&city, "city", "", "City")
	cmd.Flags().StringVar(&state, "state", "", "State or province code")
	cmd.Flags().StringVar(&zip, "zip", "", "Postal code")
	cmd.Flags().StringVar(&country, "country", "US", "ISO country code")
	cmd.Flags().BoolVar(&useCache, "cache", true, "Consult local cache before calling the API")
	return cmd
}

// addressCacheKey hashes the normalized address fields. Using SHA-256 of the
// lower-cased, trimmed concatenation gives stable cache keys across whitespace
// and case differences without leaking the raw address into logs.
func addressCacheKey(street, city, state, zip, country string) string {
	norm := func(s string) string { return strings.ToLower(strings.TrimSpace(s)) }
	joined := strings.Join([]string{norm(street), norm(city), norm(state), norm(zip), norm(country)}, "|")
	sum := sha256.Sum256([]byte(joined))
	return hex.EncodeToString(sum[:])
}

func parseAddressResolve(data json.RawMessage) (classification, street, city, state, postal string) {
	var resp struct {
		Output struct {
			ResolvedAddresses []struct {
				Classification string   `json:"classification"`
				StreetLines    []string `json:"streetLinesToken"`
				StreetLines2   []string `json:"streetLines"`
				City           string   `json:"city"`
				State          string   `json:"stateOrProvinceCode"`
				Postal         string   `json:"postalCode"`
			} `json:"resolvedAddresses"`
		} `json:"output"`
	}
	_ = json.Unmarshal(data, &resp)
	if len(resp.Output.ResolvedAddresses) == 0 {
		return
	}
	r := resp.Output.ResolvedAddresses[0]
	classification = r.Classification
	city = r.City
	state = r.State
	postal = r.Postal
	if len(r.StreetLines2) > 0 {
		street = r.StreetLines2[0]
	} else if len(r.StreetLines) > 0 {
		street = r.StreetLines[0]
	}
	return
}

func addressValidationToJSON(av *store.AddressValidationCache, cacheHit bool) map[string]any {
	return map[string]any{
		"cache_hit":      cacheHit,
		"classification": av.Classification,
		"resolved": map[string]any{
			"street": av.ResolvedStreet,
			"city":   av.ResolvedCity,
			"state":  av.ResolvedState,
			"postal": av.ResolvedPostal,
		},
	}
}
