// Hand-written (not generated): version-checked, non-migrating local report access.
package cli

import (
	"context"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/store"
)

// etsyOpenReadOnlyChecked accepts older schemas without migrating and rejects newer ones.
func etsyOpenReadOnlyChecked(ctx context.Context, path string) (*store.Store, error) {
	db, err := store.OpenReadOnlyContext(ctx, path)
	if err != nil {
		return nil, err
	}
	version, err := db.SchemaVersion()
	if err == nil && version > store.StoreSchemaVersion {
		err = fmt.Errorf("database schema version %d is newer than supported version %d; upgrade the CLI binary or open an older database", version, store.StoreSchemaVersion)
	}
	if err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}
