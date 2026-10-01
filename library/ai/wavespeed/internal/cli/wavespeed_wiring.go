// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source auto

package cli

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/ai/wavespeed/internal/cliutil"
)

// The generated root reserves one newNovel<Name>Cmd slot per research novel
// feature. The hand-authored commands register themselves in the hook below;
// these slots are marked as scaffolds so preferImplementedNovelCommands drops
// them in favor of the real command with the same name.
func novelSlot(use string) *cobra.Command {
	return &cobra.Command{Use: use, Hidden: true, Annotations: map[string]string{novelScaffoldAnnotation: "true"}}
}

func newNovelRunCmd(*rootFlags) *cobra.Command      { return novelSlot("run") }
func newNovelSchemaCmd(*rootFlags) *cobra.Command   { return novelSlot("schema") }
func newNovelPriceCmd(*rootFlags) *cobra.Command    { return novelSlot("price") }
func newNovelUploadCmd(*rootFlags) *cobra.Command   { return novelSlot("upload") }
func newNovelDownloadCmd(*rootFlags) *cobra.Command { return novelSlot("download") }
func newNovelLastCmd(*rootFlags) *cobra.Command     { return novelSlot("last") }
func newNovelAliasesCmd(*rootFlags) *cobra.Command  { return novelSlot("aliases") }
func newNovelInitCmd(*rootFlags) *cobra.Command     { return novelSlot("init") }
func newNovelPlanCmd(*rootFlags) *cobra.Command     { return novelSlot("plan") }
func newNovelQaCmd(*rootFlags) *cobra.Command       { return novelSlot("qa") }
func newNovelPackCmd(*rootFlags) *cobra.Command     { return novelSlot("pack") }
func newNovelBatchCmd(*rootFlags) *cobra.Command    { return novelSlot("batch") }
func newNovelVariantsCmd(*rootFlags) *cobra.Command { return novelSlot("variants") }
func newNovelComposeCmd(*rootFlags) *cobra.Command  { return novelSlot("compose") }
func newNovelAspectsCmd(*rootFlags) *cobra.Command  { return novelSlot("aspects") }
func newNovelRestyleCmd(*rootFlags) *cobra.Command  { return novelSlot("restyle") }
func newNovelLibraryCmd(*rootFlags) *cobra.Command  { return novelSlot("library") }
func newNovelBrandCmd(*rootFlags) *cobra.Command    { return novelSlot("brand") }

// wavespeedCommandAliases mirrors the official WaveSpeed CLI vocabulary on top
// of the generated endpoint names; existing scripts call both forms.
var wavespeedCommandAliases = map[string][]string{
	"account-balance":      {"balance"},
	"predictions":          {"history"},
	"prediction-results":   {"show"},
	"prediction-deletions": {"delete"},
}

func init() {
	registerNovelCommand(func(rootCmd *cobra.Command, flags *rootFlags) {
		rootCmd.AddCommand(newRunCmd(flags))
		rootCmd.AddCommand(newSchemaCmd(flags))
		rootCmd.AddCommand(newPriceCmd(flags))
		rootCmd.AddCommand(newUploadCmd(flags))
		rootCmd.AddCommand(newDownloadCmd(flags))
		rootCmd.AddCommand(newLastCmd(flags))
		rootCmd.AddCommand(newOpenCmd(flags))
		rootCmd.AddCommand(newAliasesCmd(flags))
		rootCmd.AddCommand(newInitCmd(flags))
		rootCmd.AddCommand(newPlanCmd(flags))
		rootCmd.AddCommand(newQACmd(flags))
		rootCmd.AddCommand(newPackCmd(flags))
		rootCmd.AddCommand(newBatchCmd(flags))
		rootCmd.AddCommand(newVariantsCmd(flags))
		rootCmd.AddCommand(newComposeCmd(flags))
		rootCmd.AddCommand(newAspectsCmd(flags))
		rootCmd.AddCommand(newRestyleCmd(flags))
		rootCmd.AddCommand(newLibraryCmd(flags))
		rootCmd.AddCommand(newBrandCmd(flags))
		for _, child := range rootCmd.Commands() {
			for _, alias := range wavespeedCommandAliases[child.Name()] {
				if !child.HasAlias(alias) {
					child.Aliases = append(child.Aliases, alias)
				}
			}
		}
	})
}

// archiveDBPath is the generated sync store that novel commands read for
// cached pricing. WAVESPEED_ARCHIVE_DB overrides it (tests and relocated
// installs); otherwise it is the same data.db the generated sync writes.
func archiveDBPath() string {
	if env := strings.TrimSpace(os.Getenv("WAVESPEED_ARCHIVE_DB")); env != "" {
		return env
	}
	return defaultDBPath("wavespeed-pp-cli")
}

// libraryDBFile is the library database file name inside the data dir.
const libraryDBFile = "library" + ".db"

// libraryDBPath is the D2C generation library database. It lives beside the
// generated sync store (data.db) under the CLI's data directory and keeps an
// independent schema version. WAVESPEED_LIBRARY_DB overrides the path.
func libraryDBPath() string {
	if env := strings.TrimSpace(os.Getenv("WAVESPEED_LIBRARY_DB")); env != "" {
		return env
	}
	if dir, err := cliutil.DataDir(); err == nil {
		return filepath.Join(dir, libraryDBFile)
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "wavespeed-pp-cli", libraryDBFile)
}

// partialFailureErr marks a completed paid run whose follow-up step (a local
// download) failed. The generation is kept and recorded, so callers get exit 6
// with a partial_failure envelope rather than a generic API error.
func partialFailureErr(err error) error { return &cliError{code: 6, err: err} }

// printRunOutput prints hand-authored run/price/schema/upload/download output
// the way the published CLI always has: the payload itself, not the generated
// {meta, results} envelope. Scripts parse these documents directly (for
// example `price ... --agent | jq .data.unit_price`), so the shape is part of
// the CLI contract. --select, --compact, --quiet, and --csv still apply.
func printRunOutput(w io.Writer, data json.RawMessage, flags *rootFlags) error {
	if flags.selectFields != "" {
		data = filterFields(data, flags.selectFields)
	} else if flags.compact {
		data = compactFields(data)
	}
	if flags.quiet {
		return nil
	}
	if flags.csv {
		return printCSV(w, data)
	}
	return printOutput(w, data, flags.asJSON)
}
