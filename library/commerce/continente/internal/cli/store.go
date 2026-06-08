package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"continente-pp-cli/internal/acquisition/storefront"
	"continente-pp-cli/internal/config"
	"github.com/spf13/cobra"
)

func newStoreCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "store",
		Short: "Discover and manage preferred store context",
		Long:  "Discover nearby pickup stores and persist a preferred store for future agent or operator workflows.",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newStoreNearbyCmd(flags))
	cmd.AddCommand(newStoreCurrentCmd(flags))
	cmd.AddCommand(newStoreSetCmd(flags))
	cmd.AddCommand(newStoreClearCmd(flags))
	return cmd
}

func newStoreNearbyCmd(flags *rootFlags) *cobra.Command {
	var latitude float64
	var longitude float64
	var limit int

	cmd := &cobra.Command{
		Use:         "nearby",
		Short:       "List nearby stores from coordinates",
		Example:     "  continente-pp-cli store nearby --lat 38.7527 --lng -9.1848 --limit 10",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			preferredStore, err := loadPreferredStore(flags)
			if err != nil {
				return configErr(err)
			}
			latitude, longitude, err = resolveStoreLookupCoordinates(latitude, longitude, preferredStore)
			if err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := storefront.FetchNearbyStores(cmd.Context(), c, latitude, longitude)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if flags.dryRun {
				return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
			}
			var payload storefront.NearbyStoresResponse
			if err := json.Unmarshal(data, &payload); err != nil {
				return err
			}
			if limit > 0 && len(payload.Stores) > limit {
				payload.Stores = payload.Stores[:limit]
			}
			rows := make([]map[string]any, 0, len(payload.Stores))
			for _, store := range payload.Stores {
				rows = append(rows, map[string]any{
					"id":              store.ID,
					"name":            store.Name,
					"city":            store.City,
					"postal_code":     store.PostalCode,
					"latitude":        store.Latitude,
					"longitude":       store.Longitude,
					"is_pickup_store": store.IsPickupStore,
				})
			}
			return emitStructuredOutput(cmd, flags, payload, DataProvenance{Source: "live", ResourceType: "stores"}, len(payload.Stores), rows)
		},
	}
	cmd.Flags().Float64Var(&latitude, "lat", 0, "Latitude")
	cmd.Flags().Float64Var(&longitude, "lng", 0, "Longitude")
	cmd.Flags().IntVar(&limit, "limit", 0, "Limit results")
	return cmd
}

func newStoreCurrentCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "current",
		Short:       "Show the persisted preferred store",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			if cfg.PreferredStore == nil {
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"preferred_store": nil}, flags)
				}
				_, err := fmt.Fprintln(cmd.OutOrStdout(), "no preferred store configured")
				return err
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"preferred_store": cfg.PreferredStore}, flags)
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "%s (%s)\n", cfg.PreferredStore.Name, cfg.PreferredStore.ID)
			return err
		},
	}
}

func newStoreSetCmd(flags *rootFlags) *cobra.Command {
	var id string
	var name string
	var city string
	var postalCode string
	var latitude float64
	var longitude float64
	var resolveLat float64
	var resolveLng float64

	cmd := &cobra.Command{
		Use:   "set",
		Short: "Persist the preferred store",
		Long:  "Persist a preferred store directly or resolve it from a nearby-store lookup by ID.",
		Example: "  continente-pp-cli store set --id col-1981-store --resolve-lat 38.7527 --resolve-lng -9.1848\n" +
			"  continente-pp-cli store set --id col-1981-store --name 'Continente Bom Dia São Marcos' --postal-code 2735-529 --city 'São Marcos' --lat 38.751305 --lng -9.301209",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(id) == "" {
				return usageErr(fmt.Errorf("--id is required"))
			}
			store := &config.PreferredStore{
				ID:         strings.TrimSpace(id),
				Name:       strings.TrimSpace(name),
				City:       strings.TrimSpace(city),
				PostalCode: strings.TrimSpace(postalCode),
				Latitude:   latitude,
				Longitude:  longitude,
			}
			if needsStoreResolution(store) {
				if resolveLat == 0 || resolveLng == 0 {
					return usageErr(fmt.Errorf("missing store details; pass --resolve-lat and --resolve-lng or provide --name, --postal-code, --city, --lat, and --lng"))
				}
				c, err := flags.newClient()
				if err != nil {
					return err
				}
				data, err := storefront.FetchNearbyStores(cmd.Context(), c, resolveLat, resolveLng)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				var payload storefront.NearbyStoresResponse
				if err := json.Unmarshal(data, &payload); err != nil {
					return err
				}
				record, ok := findStoreByID(payload.Stores, store.ID)
				if !ok {
					return notFoundErr(fmt.Errorf("store %q not found in nearby results", store.ID))
				}
				store.Name = record.Name
				store.City = record.City
				store.PostalCode = record.PostalCode
				store.Latitude = record.Latitude
				store.Longitude = record.Longitude
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			cfg.PreferredStore = store
			if err := cfg.Save(); err != nil {
				return configErr(err)
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"preferred_store": store}, flags)
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "saved preferred store %q (%s)\n", store.Name, store.ID)
			return err
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "Store ID")
	cmd.Flags().StringVar(&name, "name", "", "Store name")
	cmd.Flags().StringVar(&city, "city", "", "Store city")
	cmd.Flags().StringVar(&postalCode, "postal-code", "", "Store postal code")
	cmd.Flags().Float64Var(&latitude, "lat", 0, "Store latitude")
	cmd.Flags().Float64Var(&longitude, "lng", 0, "Store longitude")
	cmd.Flags().Float64Var(&resolveLat, "resolve-lat", 0, "Lookup latitude used to resolve store details by ID")
	cmd.Flags().Float64Var(&resolveLng, "resolve-lng", 0, "Lookup longitude used to resolve store details by ID")
	return cmd
}

func newStoreClearCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "clear",
		Short: "Clear the preferred store",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			cfg.PreferredStore = nil
			if err := cfg.Save(); err != nil {
				return configErr(err)
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"preferred_store": nil}, flags)
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), "cleared preferred store")
			return err
		},
	}
}

func needsStoreResolution(store *config.PreferredStore) bool {
	if store == nil {
		return true
	}
	return store.Name == "" || store.City == "" || store.PostalCode == "" || store.Latitude == 0 || store.Longitude == 0
}

func findStoreByID(stores []storefront.StoreRecord, id string) (storefront.StoreRecord, bool) {
	id = strings.TrimSpace(id)
	for _, store := range stores {
		if strings.TrimSpace(store.ID) == id {
			return store, true
		}
	}
	return storefront.StoreRecord{}, false
}
