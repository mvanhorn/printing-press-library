// Copyright 2026 Adrian Horning and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: snapshot+diff watcher for Instagram tagged posts (UGC).
// pp:data-source auto

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/internal/store"
)

type taggedEnvelope struct {
	Handle         string   `json:"handle"`
	UserID         string   `json:"user_id"`
	Current        int      `json:"current_tagged_posts"`
	New            []string `json:"new_post_ids"`
	LeftLatestPage []string `json:"left_latest_page_post_ids"`
	FirstSnapshot  bool     `json:"first_snapshot"`
	CreditsCharged int64    `json:"credits_charged"`
}

func newNovelCreatorTaggedCmd(flags *rootFlags) *cobra.Command {
	var userID string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "tagged [handle]",
		Short: "Snapshot the latest page of posts a creator is tagged in and diff new mentions on rerun",
		Example: strings.Trim(`
  scrape-creators-pp-cli creator tagged bracken.design --agent
  scrape-creators-pp-cli creator tagged --user-id 123456 --agent`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "false", "pp:happy-args": "handle=mock-value"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would snapshot tagged posts and diff against the previous run")
				return nil
			}
			handle := ""
			if len(args) > 0 {
				handle = args[0]
			}
			if handle == "" && userID == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a handle or --user-id is required"))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			env := taggedEnvelope{Handle: handle, UserID: userID}

			if env.UserID == "" {
				profRaw, perr := c.Get(ctx, "/v1/instagram/profile", map[string]string{"handle": handle})
				if perr != nil {
					return fmt.Errorf("resolving handle to user_id: %w", perr)
				}
				env.CreditsCharged += payloadCredits(profRaw)
				env.UserID = extractUserID(profRaw)
				if env.UserID == "" {
					return fmt.Errorf("profile response for %q carried no user id; pass --user-id explicitly", handle)
				}
			}

			tagRaw, terr := c.Get(ctx, "/v1/instagram/user/tagged-posts", map[string]string{"user_id": env.UserID})
			if terr != nil {
				return fmt.Errorf("fetching tagged posts: %w", terr)
			}
			if isErrorEnvelope(tagRaw) {
				return fmt.Errorf("tagged-posts endpoint returned an error envelope for user %s", env.UserID)
			}
			env.CreditsCharged += payloadCredits(tagRaw)
			ids := extractTaggedIDs(tagRaw)
			env.Current = len(ids)

			key := handle
			if key == "" {
				key = env.UserID
			}
			if dbPath == "" {
				dbPath = defaultDBPath("scrape-creators-pp-cli")
			}
			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			if err := store.EnsureTaggedSnapshots(ctx, db.DB()); err != nil {
				return fmt.Errorf("tagged snapshot migration: %w", err)
			}
			_, prev, err := store.LatestTaggedSnapshot(ctx, db.DB(), key)
			if err != nil {
				return fmt.Errorf("reading previous snapshot: %w", err)
			}
			if prev == nil {
				env.FirstSnapshot = true
			} else {
				prevSet := make(map[string]bool, len(prev))
				for _, id := range prev {
					prevSet[id] = true
				}
				curSet := make(map[string]bool, len(ids))
				for _, id := range ids {
					curSet[id] = true
					if !prevSet[id] {
						env.New = append(env.New, id)
					}
				}
				for _, id := range prev {
					if !curSet[id] {
						// The tagged endpoint is read one page at a time, so an
						// id leaving the latest page may just have been pushed
						// off by newer tags, not deleted.
						env.LeftLatestPage = append(env.LeftLatestPage, id)
					}
				}
			}
			if err := store.InsertTaggedSnapshot(ctx, db.DB(), key, ids, time.Now()); err != nil {
				return fmt.Errorf("writing snapshot: %w", err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), env, flags)
		},
	}
	cmd.Flags().StringVar(&userID, "user-id", "", "Instagram user ID (skips the handle-to-ID profile call)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// extractUserID pulls the numeric user id out of a profile envelope.
func extractUserID(raw json.RawMessage) string {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return ""
	}
	for _, wrap := range []string{"", "data", "user"} {
		src := obj
		if wrap != "" {
			v, ok := obj[wrap]
			if !ok {
				continue
			}
			var nested map[string]json.RawMessage
			if err := json.Unmarshal(v, &nested); err != nil {
				continue
			}
			src = nested
		}
		for _, k := range []string{"id", "pk", "user_id"} {
			if v, ok := src[k]; ok {
				var s string
				if err := json.Unmarshal(v, &s); err == nil && s != "" {
					return s
				}
				var n json.Number
				if err := json.Unmarshal(v, &n); err == nil && n.String() != "" && n.String() != "0" {
					return n.String()
				}
			}
		}
	}
	return ""
}

// extractTaggedIDs pulls post IDs from the tagged-posts envelope.
func extractTaggedIDs(raw json.RawMessage) []string {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil
	}
	var arr []json.RawMessage
	for _, key := range []string{"posts", "items", "data", "medias"} {
		if v, ok := obj[key]; ok {
			if err := json.Unmarshal(v, &arr); err == nil && len(arr) > 0 {
				break
			}
		}
	}
	out := make([]string, 0, len(arr))
	for _, it := range arr {
		var m map[string]json.RawMessage
		if err := json.Unmarshal(it, &m); err != nil {
			continue
		}
		for _, k := range []string{"id", "pk", "code"} {
			if v, ok := m[k]; ok {
				var s string
				if err := json.Unmarshal(v, &s); err == nil && s != "" {
					out = append(out, s)
					break
				}
				var n json.Number
				if err := json.Unmarshal(v, &n); err == nil && n.String() != "" {
					out = append(out, n.String())
					break
				}
			}
		}
	}
	return out
}
