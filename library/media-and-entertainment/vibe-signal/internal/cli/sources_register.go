// Copyright 2026 not0xjarvis and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: registers the aggregator sources via their package init().

package cli

import (
	// Blank imports run each source package's init(), which registers it with
	// internal/source. Add a line here when wiring a new source.
	_ "github.com/mvanhorn/printing-press-library/library/media-and-entertainment/vibe-signal/internal/source/hackernews"
	_ "github.com/mvanhorn/printing-press-library/library/media-and-entertainment/vibe-signal/internal/source/techmeme"
)
