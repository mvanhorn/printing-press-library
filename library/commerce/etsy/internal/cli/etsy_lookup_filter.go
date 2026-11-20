package cli

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/learn"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/learn/entities"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/store"
)

var etsyPurgeOpenRW = store.OpenWithContext

// EtsyPurgeBuyerLookupsAtStart removes legacy synced buyer lookups from an existing store.
func EtsyPurgeBuyerLookupsAtStart() {
	etsyPath, etsyErr := MCPStorePath()
	if etsyErr != nil {
		return
	}
	if _, etsyErr = os.Stat(etsyPath); etsyErr != nil {
		return
	}
	etsyCtx, etsyCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer etsyCancel()
	etsyReadOnly, etsyErr := store.OpenReadOnlyContext(etsyCtx, etsyPath)
	if etsyErr != nil {
		fmt.Fprintf(os.Stderr, "warning: lookup purge skipped: %v\n", etsyErr)
		return
	}
	etsyMarks, etsyArgs := etsyBuyerLookupAllowlist()
	var etsyFound int
	etsyErr = etsyReadOnly.DB().QueryRowContext(etsyCtx, "SELECT 1 FROM entity_lookups WHERE source = 'synced' AND kind NOT IN ("+etsyMarks+") LIMIT 1", etsyArgs...).Scan(&etsyFound)
	etsyCloseErr := etsyReadOnly.Close()
	etsyCancel()
	if etsyCloseErr != nil {
		fmt.Fprintf(os.Stderr, "warning: lookup purge skipped: %v\n", etsyCloseErr)
		return
	}
	if errors.Is(etsyErr, sql.ErrNoRows) || (etsyErr != nil && strings.Contains(etsyErr.Error(), "no such table")) {
		return
	}
	if etsyErr != nil {
		fmt.Fprintf(os.Stderr, "warning: lookup purge skipped: %v\n", etsyErr)
		return
	}
	etsyCtx, etsyCancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer etsyCancel()
	etsyDB, etsyErr := etsyPurgeOpenRW(etsyCtx, etsyPath)
	if etsyErr != nil {
		fmt.Fprintf(os.Stderr, "warning: lookup purge skipped: %v\n", etsyErr)
		return
	}
	if etsyErr = etsyPurgeBuyerLookups(etsyCtx, etsyDB.DB()); etsyErr != nil {
		fmt.Fprintf(os.Stderr, "warning: lookup purge skipped: %v\n", etsyErr)
	}
	if etsyErr = etsyDB.Close(); etsyErr != nil {
		fmt.Fprintf(os.Stderr, "warning: lookup purge skipped: %v\n", etsyErr)
	}
}

func etsyLookupExtractor(ctx context.Context, db *sql.DB, cfg *entities.Config) func(string, []byte) []string {
	if err := etsyPurgeBuyerLookups(ctx, db); err != nil {
		fmt.Fprintf(os.Stderr, "warning: lookup purge skipped: %v\n", err)
	}
	return func(resourceType string, data []byte) []string {
		if strings.TrimSpace(resourceType) == "" || etsyBuyerBearingType(resourceType) {
			return nil
		}
		return learn.ResourceEntitiesFromJSON(data, lookupRefreshEntityFields, cfg)
	}
}

func etsyBuyerLookupAllowlist() (string, []any) {
	etsyTypes := EtsyNonBuyerResourceTypes()
	etsyMarks := make([]string, len(etsyTypes))
	etsyArgs := make([]any, len(etsyTypes))
	for i, etsyType := range etsyTypes {
		etsyMarks[i] = "?"
		etsyArgs[i] = etsyType
	}
	return strings.Join(etsyMarks, ","), etsyArgs
}

func etsyPurgeBuyerLookups(ctx context.Context, db *sql.DB) error {
	etsyMarks, etsyArgs := etsyBuyerLookupAllowlist()
	_, err := db.ExecContext(ctx, "DELETE FROM entity_lookups WHERE source = 'synced' AND kind NOT IN ("+etsyMarks+")", etsyArgs...) // #nosec G202 -- only ? placeholders are concatenated; values are bound args.
	if err != nil && strings.Contains(err.Error(), "no such table") {
		return nil
	}
	return err
}
