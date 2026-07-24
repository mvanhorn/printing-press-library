// Copyright 2026 togorashi45 and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/arcgis/internal/store"
)

// ensureFeaturesView creates the `features` SQL view over synced feature rows so
// that `sql` can SELECT ... FROM features with the columns layer_url, oid,
// attributes (a JSON string), and geometry (a JSON string).
func ensureFeaturesView(ctx context.Context, db *store.Store) error {
	const ddl = `
CREATE VIEW IF NOT EXISTS features AS
SELECT
  json_extract(data, '$.layer_url') AS layer_url,
  json_extract(data, '$.oid')       AS oid,
  json_extract(data, '$.attributes') AS attributes,
  json_extract(data, '$.geometry')   AS geometry
FROM resources
WHERE resource_type = 'feature';`
	if _, err := db.DB().ExecContext(ctx, ddl); err != nil {
		return fmt.Errorf("creating features view: %w", err)
	}
	return nil
}
