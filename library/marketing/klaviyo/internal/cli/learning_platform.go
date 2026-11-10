// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/learn"
	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/platform"
	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/store"
)

func bindLearningState(session *platform.Session) (func(), error) {
	if session == nil || session.GateOutcome != platform.GateVerified || strings.TrimSpace(session.Paths.StateDir) == "" {
		return nil, fmt.Errorf("verified client profile state directory is required")
	}
	return learn.BindStateDir(session.Paths.StateDir), nil
}

func openLearningStore(ctx context.Context, explicit string) (*store.Store, error) {
	session := platform.SessionFromContext(ctx)
	if session == nil || session.GateOutcome != platform.GateVerified || strings.TrimSpace(session.Paths.DataFile) == "" {
		return nil, fmt.Errorf("verified client profile database is required")
	}
	if explicit != "" && filepath.Clean(explicit) != filepath.Clean(session.Paths.DataFile) {
		return nil, fmt.Errorf("tenant artifact conflict: --db must use the selected client profile path")
	}
	return store.OpenVerifiedWithContext(ctx, session.Paths.DataFile, session)
}
