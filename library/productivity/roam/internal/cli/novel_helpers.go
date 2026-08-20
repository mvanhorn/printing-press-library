package cli

import (
	"context"
	"database/sql"
	"fmt"
)

// openNovelDB opens the local store DB for novel commands. Errors with a clear
// message if the user hasn't synced yet (DB file absent).
func openNovelDB(ctx context.Context, flags *rootFlags) (*sql.DB, func(), error) {
	_ = flags
	s, err := openStoreForRead(ctx, "roam-pp-cli")
	if err != nil {
		return nil, func() {}, configErr(fmt.Errorf("open store: %w", err))
	}
	if s == nil {
		return nil, func() {}, configErr(fmt.Errorf("local store not initialized — run 'roam-pp-cli sync' first"))
	}
	return s.DB(), func() { _ = s.Close() }, nil
}
