// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/human-goat/internal/source/magic"
	"github.com/mvanhorn/printing-press-library/library/productivity/human-goat/internal/store"
)

// persistMagicRequest records a Magic request in the local store so `status`
// can surface it as part of the cross-source inbox (the Magic API has no
// list-requests endpoint, so the local record is the only inbox source).
// Best-effort: the request already succeeded remotely, so a store failure must
// never fail the user-facing command.
func persistMagicRequest(ctx context.Context, req *magic.Request) {
	if req == nil || strings.TrimSpace(req.ID) == "" {
		return
	}
	data, err := json.Marshal(req)
	if err != nil {
		return
	}
	db, err := store.OpenWithContext(ctx, defaultDBPath("human-goat-pp-cli"))
	if err != nil {
		return
	}
	defer db.Close()
	_ = db.Upsert("magic", req.ID, data)
}
