// Copyright 2026 Zain Haseeb and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written novel feature; not generated.

package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/skool/internal/tiptap"
)

// markdownBodyFlags holds the two ways a caller supplies a body as Markdown
// instead of hand-written TipTap JSON. They are mutually exclusive, and either
// one replaces --content.
type markdownBodyFlags struct {
	md     string // inline Markdown
	fromMD string // path to a Markdown file, or "-" for stdin
}

func (f *markdownBodyFlags) set() bool {
	return f.md != "" || f.fromMD != ""
}

// resolve returns the TipTap JSON string for whichever flag was supplied.
// It returns ("", nil) when neither is set, so callers can fall through to
// --content.
func (f *markdownBodyFlags) resolve(stdin func() ([]byte, error)) (string, error) {
	if !f.set() {
		// Neither flag supplied: the caller falls through to --content.
		// Without this guard an unset pair reaches the empty-source check
		// below, which fails an ordinary --content call.
		return "", nil
	}
	if f.md != "" && f.fromMD != "" {
		return "", usageErr(fmt.Errorf("--md and --from-md are mutually exclusive"))
	}
	src := f.md
	if f.fromMD != "" {
		var (
			raw []byte
			err error
		)
		if f.fromMD == "-" {
			raw, err = stdin()
			if err != nil {
				return "", fmt.Errorf("reading markdown from stdin: %w", err)
			}
		} else {
			raw, err = os.ReadFile(f.fromMD)
			if err != nil {
				return "", fmt.Errorf("reading markdown file: %w", err)
			}
		}
		src = string(raw)
	}
	if strings.TrimSpace(src) == "" {
		return "", usageErr(fmt.Errorf("markdown body is empty"))
	}
	return tiptap.JSONFromMarkdown(src)
}
