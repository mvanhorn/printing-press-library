package cli

import (
	"context"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/drudgereport/internal/drudge"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/drudgereport/internal/store"
)

// fetchDrudge opens the local store (ensuring drudge schema), fetches the
// live Drudge HTML (with RSS fallback), persists a snapshot + per-story
// rows + slot events, and returns the parsed stories. Used by splash,
// breaking, headlines. Honors verify-mode (drudge.FetchHTML returns
// embedded samples under PRINTING_PRESS_VERIFY=1).
func fetchDrudge(ctx context.Context) (snapshotID string, stories []drudge.Story, events []drudge.SlotEvent, err error) {
	dbPath := defaultDBPath("drudgereport-pp-cli")
	s, openErr := store.OpenWithContext(ctx, dbPath)
	if openErr != nil {
		return "", nil, nil, fmt.Errorf("open store: %w", openErr)
	}
	defer s.Close()
	if migErr := store.EnsureDrudgeSchema(ctx, s.DB()); migErr != nil {
		return "", nil, nil, fmt.Errorf("ensure drudge schema: %w", migErr)
	}
	return drudge.FetchAndPersist(ctx, s.DB(), drudge.DefaultHTMLURL, drudge.DefaultRSSURL)
}
