package cli

import (
	"fmt"

	"continente-pp-cli/internal/config"
)

func applyPreferredStoreProvenance(flags *rootFlags, prov DataProvenance) DataProvenance {
	if prov.Store != nil {
		return prov
	}
	store, err := loadPreferredStore(flags)
	if err != nil || store == nil {
		return prov
	}
	prov.Store = store
	return prov
}

func loadPreferredStore(flags *rootFlags) (*config.PreferredStore, error) {
	configPath := ""
	if flags != nil {
		configPath = flags.configPath
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, err
	}
	return cfg.PreferredStore, nil
}

func resolveStoreLookupCoordinates(latitude, longitude float64, preferred *config.PreferredStore) (float64, float64, error) {
	switch {
	case latitude != 0 && longitude != 0:
		return latitude, longitude, nil
	case latitude != 0 || longitude != 0:
		return 0, 0, usageErr(fmt.Errorf("--lat and --lng must be provided together"))
	case preferred == nil:
		return 0, 0, usageErr(fmt.Errorf("--lat and --lng are required when no preferred store is configured"))
	case preferred.Latitude == 0 || preferred.Longitude == 0:
		return 0, 0, usageErr(fmt.Errorf("preferred store %q is missing coordinates; pass --lat and --lng explicitly or re-save the store with coordinates", preferred.ID))
	default:
		return preferred.Latitude, preferred.Longitude, nil
	}
}
