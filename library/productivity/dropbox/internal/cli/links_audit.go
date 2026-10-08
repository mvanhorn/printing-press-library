// pp:data-source auto
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/dropbox"
	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/store"
	"github.com/spf13/cobra"
)

type auditLink struct {
	URL        string   `json:"url"`
	Path       string   `json:"path"`
	Name       string   `json:"name"`
	Visibility string   `json:"visibility"`
	Expires    string   `json:"expires"`
	Flags      []string `json:"flags"`
	LinkType   string   `json:"-"`
	ID         string   `json:"-"`
}
type auditCounts struct {
	Total      int `json:"total"`
	Public     int `json:"public"`
	NoExpiry   int `json:"no_expiry"`
	Dangling   int `json:"dangling"`
	StaleIndex int `json:"stale_index"`
	Unknown    int `json:"unknown"`
}
type auditUser struct {
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	AccessType  string `json:"access_type"`
}
type auditGroup struct {
	Name       string `json:"name"`
	AccessType string `json:"access_type"`
}
type auditInvitee struct {
	Email      string `json:"email"`
	AccessType string `json:"access_type"`
}
type auditFolder struct {
	Name       string         `json:"name"`
	Path       string         `json:"path"`
	AccessType string         `json:"access_type"`
	Users      []auditUser    `json:"users"`
	Groups     []auditGroup   `json:"groups"`
	Invitees   []auditInvitee `json:"invitees"`
	ID         string         `json:"-"`
}
type linksAuditResult struct {
	planOutput
	Links          []auditLink   `json:"links"`
	Counts         auditCounts   `json:"counts"`
	SharedFolders  []auditFolder `json:"shared_folders"`
	FoldersScanned int           `json:"folders_scanned"`
	FoldersTotal   int           `json:"folders_total"`
	Source         string        `json:"source"`
}

func newNovelLinksAuditCmd(flags *rootFlags) *cobra.Command {
	var dbPath, planPath string
	var revoke []string
	var maxFolders int
	var force, printPlan bool
	cmd := &cobra.Command{Use: "audit", Short: "Audit shared links and shared folder membership", Long: "Refresh shared links, flag public, non-expiring, and dangling links, and inspect shared folder membership. Write a reviewable revocation plan with --plan.", Example: strings.Trim(`
  dropbox-pp-cli links audit --agent
  dropbox-pp-cli links audit --revoke public --plan links.json --agent`, "\n"), Args: cobra.NoArgs,
		Annotations: map[string]string{"mcp:write-flags": "plan", "pp:data-source": "auto", "pp:preview-happy-path": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "links audit")
			}
			if maxFolders < 0 {
				return usageErr(fmt.Errorf("--max-folders must be nonnegative"))
			}
			if flags.dataSource != "" && flags.dataSource != "auto" && flags.dataSource != "local" && flags.dataSource != "live" {
				return usageErr(fmt.Errorf("invalid data source for links audit"))
			}
			if len(revoke) == 0 {
				revoke = []string{"dangling"}
			}
			selected := map[string]bool{}
			for _, v := range revoke {
				if v != "dangling" && v != "public" && v != "no-expiry" {
					return usageErr(fmt.Errorf("--revoke must be dangling, public, or no-expiry"))
				}
				selected[strings.ReplaceAll(v, "-", "_")] = true
			}
			if cliutil.IsDogfoodEnv() && maxFolders > 5 {
				maxFolders = 5
			}
			result := linksAuditResult{Links: make([]auditLink, 0), SharedFolders: make([]auditFolder, 0), Source: "cache"}
			ctx := cmd.Context()
			cancel := func() {}
			if cmd.Flags().Changed("timeout") {
				ctx, cancel = boundCtx(ctx, flags)
			}
			defer cancel()
			db, found, err := openIndex(cmd, flags, dbPath)
			if err != nil {
				return err
			}
			if !found {
				if planPath != "" {
					return usageErr(fmt.Errorf("no local index; run: dropbox-pp-cli index"))
				}
				result.planOutput, err = outputPlan(planPath, "links audit", nil, force, printPlan)
				if err != nil {
					return err
				}
				return printAuditResult(cmd, flags, result)
			}
			defer db.Close()
			if flags.dataSource != "local" {
				c, liveErr := flags.newClient()
				if liveErr == nil {
					var info accountInfo
					info, liveErr = verifiedDropboxAccount(ctx, c, db)
					if liveErr != nil {
						return liveErr
					}
					if liveErr == nil {
						headers := pathRootHeaders(info)
						var complete bool
						result.Links, complete, liveErr = fetchAuditLinks(ctx, c, headers)
						if liveErr == nil {
							result.SharedFolders, result.FoldersTotal, liveErr = fetchAuditFolders(ctx, c, headers, maxFolders)
						}
						if liveErr == nil && complete {
							liveErr = replaceAuditLinks(ctx, db, result.Links)
						}
						if liveErr == nil {
							result.Source = "live"
							result.FoldersScanned = len(result.SharedFolders)
						}
					}
				}
				if liveErr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "live link audit failed; using cached links: %v\n", liveErr)
				}
			}
			if result.Source == "cache" {
				result.Links, err = cachedAuditLinks(ctx, db)
				if err != nil {
					return err
				}
				result.SharedFolders = make([]auditFolder, 0)
			}
			if err := flagAuditLinks(ctx, db, &result); err != nil {
				return err
			}
			ops := make([]dropbox.Op, 0)
			for _, link := range result.Links {
				match := false
				for _, flag := range link.Flags {
					if selected[flag] {
						match = true
					}
				}
				if match {
					expectDangling := false
					for _, flag := range link.Flags {
						if flag == "dangling" && selected[flag] {
							expectDangling = true
						}
					}
					ops = append(ops, dropbox.Op{Op: "revoke_link", URL: link.URL, Reason: "audit flags: " + strings.Join(link.Flags, ", "), ExpectDangling: expectDangling})
				}
			}
			result.planOutput, err = outputPlan(planPath, "links audit", ops, force, printPlan)
			if err != nil {
				return err
			}
			return printAuditResult(cmd, flags, result)
		}}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite index path")
	cmd.Flags().StringVar(&planPath, "plan", "", "Write a link-revocation plan")
	cmd.Flags().StringArrayVar(&revoke, "revoke", nil, "Flag to revoke: dangling, public, or no-expiry (repeatable)")
	cmd.Flags().IntVar(&maxFolders, "max-folders", 50, "Maximum shared folders to inspect")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite a file that is not a valid plan")
	cmd.Flags().BoolVar(&printPlan, "print-plan", false, "Include full plan operations in JSON output")
	return cmd
}

func printAuditResult(cmd *cobra.Command, flags *rootFlags, r linksAuditResult) error {
	if !wantsHumanTable(cmd.OutOrStdout(), flags) {
		return printJSONFiltered(cmd.OutOrStdout(), r, flags)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%d links (%d public, %d no expiry, %d dangling, %d unknown); %d/%d shared folders scanned\n", r.Counts.Total, r.Counts.Public, r.Counts.NoExpiry, r.Counts.Dangling, r.Counts.Unknown, r.FoldersScanned, r.FoldersTotal)
	for _, v := range r.Links {
		fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", v.Path, strings.Join(v.Flags, ","), v.URL)
	}
	return nil
}

func fetchAuditLinks(ctx context.Context, c *client.Client, headers map[string]string) ([]auditLink, bool, error) {
	links := make([]auditLink, 0)
	cursor := ""
	for {
		body := map[string]string{}
		if cursor != "" {
			body["cursor"] = cursor
		}
		raw, _, err := c.PostQueryWithParamsAndHeaders(ctx, "/sharing/list_shared_links", nil, body, headers)
		if err != nil {
			return nil, false, err
		}
		var page struct {
			Links []struct {
				URL             string `json:"url"`
				PathLower       string `json:"path_lower"`
				Name            string `json:"name"`
				Expires         string `json:"expires"`
				ID              string `json:"id"`
				Tag             string `json:".tag"`
				LinkPermissions struct {
					ResolvedVisibility struct {
						Tag string `json:".tag"`
					} `json:"resolved_visibility"`
				} `json:"link_permissions"`
			} `json:"links"`
			HasMore bool   `json:"has_more"`
			Cursor  string `json:"cursor"`
		}
		if err := json.Unmarshal(raw, &page); err != nil {
			return nil, false, err
		}
		for _, v := range page.Links {
			links = append(links, auditLink{URL: v.URL, Path: strings.ToLower(v.PathLower), Name: v.Name, Visibility: v.LinkPermissions.ResolvedVisibility.Tag, Expires: v.Expires, LinkType: v.Tag, ID: v.ID, Flags: make([]string, 0)})
		}
		if !page.HasMore {
			return links, true, nil
		}
		if cliutil.IsDogfoodEnv() {
			return links, false, nil
		}
		if page.Cursor == "" || page.Cursor == cursor {
			return nil, false, fmt.Errorf("list_shared_links has_more without a new cursor")
		}
		cursor = page.Cursor
	}
}

func fetchAuditFolders(ctx context.Context, c *client.Client, headers map[string]string, max int) ([]auditFolder, int, error) {
	folders := make([]auditFolder, 0)
	cursor := ""
	for {
		endpoint := "/sharing/list_folders"
		var body any = map[string]int{"limit": 1000}
		if cursor != "" {
			endpoint = "/sharing/list_folders/continue"
			body = map[string]string{"cursor": cursor}
		}
		raw, _, err := c.PostQueryWithParamsAndHeaders(ctx, endpoint, nil, body, headers)
		if err != nil {
			return nil, 0, err
		}
		var page struct {
			Entries []struct {
				Name           string `json:"name"`
				PathDisplay    string `json:"path_display"`
				SharedFolderID string `json:"shared_folder_id"`
				AccessType     struct {
					Tag string `json:".tag"`
				} `json:"access_type"`
			} `json:"entries"`
			Cursor string `json:"cursor"`
		}
		if err := json.Unmarshal(raw, &page); err != nil {
			return nil, 0, err
		}
		for _, v := range page.Entries {
			folders = append(folders, auditFolder{Name: v.Name, Path: v.PathDisplay, AccessType: v.AccessType.Tag, ID: v.SharedFolderID, Users: make([]auditUser, 0), Groups: make([]auditGroup, 0), Invitees: make([]auditInvitee, 0)})
		}
		if page.Cursor == "" || cliutil.IsDogfoodEnv() {
			break
		}
		if page.Cursor == cursor {
			return nil, 0, fmt.Errorf("list_folders cursor did not advance")
		}
		cursor = page.Cursor
	}
	total := len(folders)
	if len(folders) > max {
		folders = folders[:max]
	}
	for i := range folders {
		if folders[i].ID == "" {
			continue
		}
		memberCursor := ""
		for {
			endpoint := "/sharing/list_folder_members"
			body := map[string]string{"shared_folder_id": folders[i].ID}
			if memberCursor != "" {
				endpoint = "/sharing/list_folder_members/continue"
				body = map[string]string{"cursor": memberCursor}
			}
			raw, _, err := c.PostQueryWithParamsAndHeaders(ctx, endpoint, nil, body, headers)
			if err != nil {
				return nil, 0, err
			}
			next, err := appendAuditMembers(&folders[i], raw)
			if err != nil {
				return nil, 0, err
			}
			if next == "" || cliutil.IsDogfoodEnv() {
				break
			}
			if next == memberCursor {
				return nil, 0, fmt.Errorf("list_folder_members cursor did not advance")
			}
			memberCursor = next
		}
	}
	return folders, total, nil
}

func appendAuditMembers(folder *auditFolder, raw json.RawMessage) (string, error) {
	var p struct {
		Cursor string `json:"cursor"`
		Users  []struct {
			User struct {
				DisplayName string `json:"display_name"`
				Email       string `json:"email"`
			} `json:"user"`
			AccessType struct {
				Tag string `json:".tag"`
			} `json:"access_type"`
		} `json:"users"`
		Groups []struct {
			Group struct {
				Name string `json:"group_name"`
			} `json:"group"`
			AccessType struct {
				Tag string `json:".tag"`
			} `json:"access_type"`
		} `json:"groups"`
		Invitees []struct {
			Invitee struct {
				Email string `json:"email"`
			} `json:"invitee"`
			AccessType struct {
				Tag string `json:".tag"`
			} `json:"access_type"`
		} `json:"invitees"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return "", err
	}
	for _, v := range p.Users {
		folder.Users = append(folder.Users, auditUser{v.User.DisplayName, v.User.Email, v.AccessType.Tag})
	}
	for _, v := range p.Groups {
		folder.Groups = append(folder.Groups, auditGroup{v.Group.Name, v.AccessType.Tag})
	}
	for _, v := range p.Invitees {
		folder.Invitees = append(folder.Invitees, auditInvitee{v.Invitee.Email, v.AccessType.Tag})
	}
	return p.Cursor, nil
}

func replaceAuditLinks(ctx context.Context, db *store.Store, links []auditLink) error {
	tx, err := db.DB().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM dbx_shared_links`); err != nil {
		return err
	}
	seen := time.Now().UTC().Format(time.RFC3339)
	for _, v := range links {
		if _, err := tx.ExecContext(ctx, `INSERT INTO dbx_shared_links(url,path_lower,name,visibility,expires,link_type,id,seen_at) VALUES(?,?,?,?,?,?,?,?)`, v.URL, v.Path, v.Name, v.Visibility, v.Expires, v.LinkType, v.ID, seen); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func cachedAuditLinks(ctx context.Context, db *store.Store) ([]auditLink, error) {
	rows, err := db.DB().QueryContext(ctx, `SELECT url,COALESCE(path_lower,''),COALESCE(name,''),COALESCE(visibility,''),COALESCE(expires,''),COALESCE(link_type,''),COALESCE(id,'') FROM dbx_shared_links ORDER BY url`)
	if err != nil {
		return nil, err
	}
	links := make([]auditLink, 0)
	for rows.Next() {
		var v auditLink
		v.Flags = make([]string, 0)
		if err := rows.Scan(&v.URL, &v.Path, &v.Name, &v.Visibility, &v.Expires, &v.LinkType, &v.ID); err != nil {
			_ = rows.Close()
			return nil, err
		}
		links = append(links, v)
	}
	err = rows.Err()
	_ = rows.Close()
	return links, err
}

func flagAuditLinks(ctx context.Context, db *store.Store, r *linksAuditResult) error {
	type indexState struct{ complete, fresh bool }
	states := map[string]indexState{}
	rows, err := db.DB().QueryContext(ctx, `SELECT root,COALESCE(complete,0),COALESCE(last_full_at,''),COALESCE(last_incremental_at,'') FROM dbx_index_state`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var root string
		var complete bool
		var full, incremental string
		if err := rows.Scan(&root, &complete, &full, &incremental); err != nil {
			_ = rows.Close()
			return err
		}
		stamp := incremental
		if stamp == "" {
			stamp = full
		}
		at, parseErr := time.Parse(time.RFC3339, stamp)
		states[root] = indexState{complete: complete, fresh: parseErr == nil && time.Since(at) <= 24*time.Hour}
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return err
	}
	r.Counts = auditCounts{Total: len(r.Links)}
	for i := range r.Links {
		v := &r.Links[i]
		v.Flags = make([]string, 0)
		if v.Visibility == "public" {
			v.Flags = append(v.Flags, "public")
			r.Counts.Public++
		}
		if v.Expires == "" {
			v.Flags = append(v.Flags, "no_expiry")
			r.Counts.NoExpiry++
		}
		if v.Path == "" {
			v.Flags = append(v.Flags, "unknown")
			r.Counts.Unknown++
			continue
		}
		lower := strings.ToLower(v.Path)
		var present int
		if err := db.DB().QueryRowContext(ctx, `SELECT count(*) FROM dbx_files WHERE path_lower=?`, lower).Scan(&present); err != nil {
			return err
		}
		if present > 0 {
			continue
		}
		root := dropbox.IndexRoot(lower)
		state := states[root]
		if state.complete && state.fresh {
			v.Flags = append(v.Flags, "dangling")
			r.Counts.Dangling++
		} else if state.complete {
			v.Flags = append(v.Flags, "stale_index")
			r.Counts.StaleIndex++
		} else {
			v.Flags = append(v.Flags, "unknown")
			r.Counts.Unknown++
		}
	}
	return nil
}
