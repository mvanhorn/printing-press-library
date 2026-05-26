package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/productivity/safari-history/internal/output"
)

func renderNotAvailable(opts *RootOptions, feature, reason string) error {
	msg := fmt.Sprintf("%s not available for %s", feature, opts.Source.Name())
	if reason != "" {
		msg += ": " + reason
	}
	output.DefaultToJSONIfNotTTY(&opts.Output)
	return output.Render(opts.Output, []map[string]any{{"message": msg}})
}
