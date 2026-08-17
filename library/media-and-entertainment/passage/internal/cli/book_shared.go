// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Shared helpers for passage's practice commands (today, sit, journal, next,
// stats, shelf, log, passage).
package cli

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/passage/internal/practice"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/passage/internal/source/gutendex"
)

// workKeyRe accepts an Open Library key (OL45804W, /works/OL45804W) or a
// numeric Gutenberg id — the stable identifiers shelf/log/next key on.
var workKeyRe = regexp.MustCompile(`^(/?(works|authors|books)/)?(OL[0-9]+[A-Z]|[0-9]+)$`)

func validWorkKey(s string) bool { return workKeyRe.MatchString(strings.TrimSpace(s)) }

func atoiOr(s string, def int) int {
	if s == "" {
		return def
	}
	if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
		return n
	}
	return def
}

func shortDate(iso string) string {
	if len(iso) >= 10 {
		return iso[:10]
	}
	return iso
}

func openPractice(cmd *cobra.Command) (*practice.Practice, error) {
	return practice.Open(cmd.Context(), defaultDBPath("passage-pp-cli"))
}

func gutClient() *gutendex.Client { return gutendex.New() }

// bookRender emits typed JSON for agents/pipes and a table for humans, matching
// the generated commands' output convention.
func bookRender(cmd *cobra.Command, flags *rootFlags, jsonVal any, headers []string, rows [][]string) error {
	if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
		return flags.printJSON(cmd, jsonVal)
	}
	return flags.printTable(cmd, headers, rows)
}

// todayTopics rotates the daily pick across canonical public-domain reading.
var todayTopics = []string{
	"meditations marcus aurelius", "walden thoreau", "essays emerson",
	"montaigne essays", "leaves of grass whitman", "seneca letters",
	"tao te ching", "confessions augustine", "nature", "self-reliance",
	"the prophet", "pensees pascal",
}
