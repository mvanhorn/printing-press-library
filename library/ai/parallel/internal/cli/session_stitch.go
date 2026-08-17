// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mvanhorn/printing-press-library/library/ai/parallel/internal/store"
	"github.com/spf13/cobra"
)

func newNovelSessionStitchCmd(flags *rootFlags) *cobra.Command {
	var flagSearchID string
	var flagExtractID string
	var flagRunID string
	var flagFindallID string
	var flagSessionID string
	var flagNotes string

	cmd := &cobra.Command{
		Use:   "stitch",
		Short: "Bind search, extract, and task/findall runs into one local session chain for agent resume.",
		Example: strings.Trim(`
  parallel-pp-cli session stitch --search-id search_demo --extract-id extract_demo --json --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if err := validateDataSourceStrategy(flags, "local"); err != nil {
				return err
			}

			hasInput := flagSearchID != "" || flagExtractID != "" || flagRunID != "" || flagFindallID != ""
			if !hasInput && len(args) == 0 && !hasChangedLocalFlags(cmd) {
				return cmd.Help()
			}
			if !hasInput {
				return usageErr(fmt.Errorf("at least one of --search-id, --extract-id, --run-id, or --findall-id is required"))
			}

			sessionID := strings.TrimSpace(flagSessionID)
			if sessionID == "" {
				sessionID = uuid.NewString()
			}
			createdAt := time.Now().UTC().Format(time.RFC3339)

			members := make([]store.ResearchSessionMember, 0, 4)
			addMember := func(kind, ref string) {
				ref = strings.TrimSpace(ref)
				if ref == "" {
					return
				}
				members = append(members, store.ResearchSessionMember{Kind: kind, RefID: ref})
			}
			addMember("search", flagSearchID)
			addMember("extract", flagExtractID)
			addMember("run", flagRunID)
			addMember("findall", flagFindallID)

			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("parallel-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			if err := db.UpsertResearchSession(sessionID, createdAt, flagNotes, members); err != nil {
				return fmt.Errorf("stitch session: %w", err)
			}

			allMembers, err := db.ListResearchSessionMembers(sessionID)
			if err != nil {
				return fmt.Errorf("listing session members: %w", err)
			}

			hintIfUnsynced(cmd, db, "")
			hintIfStale(cmd, db, "", flags.maxAge)

			out := map[string]any{
				"session_id": sessionID,
				"members":    allMembers,
				"created_at": createdAt,
			}
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().StringVar(&flagSearchID, "search-id", "", "Web search run id to bind")
	cmd.Flags().StringVar(&flagExtractID, "extract-id", "", "Extract run id to bind")
	cmd.Flags().StringVar(&flagRunID, "run-id", "", "Task run id to bind")
	cmd.Flags().StringVar(&flagFindallID, "findall-id", "", "FindAll run id to bind")
	cmd.Flags().StringVar(&flagSessionID, "session-id", "", "Existing session id (generated when empty)")
	cmd.Flags().StringVar(&flagNotes, "notes", "", "Optional session notes")
	return cmd
}
