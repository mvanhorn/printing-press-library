// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/notebooklm/internal/cliutil"
	"github.com/spf13/cobra"
)

type feedbackEntry struct {
	Text      string    `json:"text"`
	CLI       string    `json:"cli"`
	Version   string    `json:"version"`
	Timestamp time.Time `json:"timestamp"`
}

func feedbackFilePath() (string, error) {
	dir, err := cliutil.DataDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return filepath.Join(dir, "feedback.jsonl"), nil
}

func newFeedbackCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "feedback",
		Short: "Record agent feedback about CLI behavior locally",
	}
	cmd.AddCommand(&cobra.Command{
		Use:     "submit <message>",
		Short:   "Append a feedback note to the local ledger",
		Args:    cobra.MinimumNArgs(1),
		Example: `  notebooklm-pp-cli feedback submit "search missed notebook by partial title" --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			entry := feedbackEntry{
				Text:      args[0],
				CLI:       "notebooklm-pp-cli",
				Version:   version,
				Timestamp: time.Now().UTC(),
			}
			p, err := feedbackFilePath()
			if err != nil {
				return err
			}
			f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
			if err != nil {
				return err
			}
			defer f.Close()
			if err := json.NewEncoder(f).Encode(entry); err != nil {
				return err
			}
			if flags.asJSON {
				return printJSON(map[string]string{"status": "recorded"})
			}
			fmt.Println("feedback recorded")
			return nil
		},
	})
	return cmd
}
