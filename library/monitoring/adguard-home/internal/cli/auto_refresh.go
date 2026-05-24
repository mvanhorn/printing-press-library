package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/mvanhorn/printing-press-library/library/monitoring/adguard-home/internal/store"
)

func autoRefreshIfStale(ctx context.Context, staleAfter time.Duration) error {
	db, err := store.OpenReadOnly("")
	if err != nil {
		return nil
	}
	defer db.Close()

	_, lastSynced, _, err := db.GetSyncState("")
	if err != nil {
		return nil
	}
	if time.Since(lastSynced) > staleAfter {
		fmt.Fprintln(os.Stderr, "Cache is stale, refreshing...")
	}
	return nil
}
