// Copyright 2026 Greg Ceccarelli and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/tella/internal/client"
)

// clipTranscriptPath is the only live transcript route. Tella removed
// /transcript/cut and /transcript/uncut (both 404); the uncut view is now
// /transcript?includeCut=true.
func clipTranscriptPath(videoID, clipID string) string {
	return fmt.Sprintf("/v1/videos/%s/clips/%s/transcript", videoID, clipID)
}

func clipTranscriptParams(includeCut bool) map[string]string {
	if includeCut {
		return map[string]string{"includeCut": "true"}
	}
	return nil
}

// getClipTranscript reads a clip transcript: playback words (cuts removed) by
// default, or every recorded word with removed ones marked `cut` when
// includeCut is true.
func getClipTranscript(ctx context.Context, c *client.Client, videoID, clipID string, includeCut bool) (json.RawMessage, error) {
	return c.Get(ctx, clipTranscriptPath(videoID, clipID), clipTranscriptParams(includeCut))
}
