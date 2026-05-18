// Copyright 2026 dstevens. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/granola"
)

func newDuplicatesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "duplicates",
		Short: "Detect duplicate meeting captures between cache and filesystem",
	}
	cmd.AddCommand(newDuplicatesScanCmd(flags))
	return cmd
}

func newDuplicatesScanCmd(flags *rootFlags) *cobra.Command {
	var root, since string
	cmd := &cobra.Command{
		Use:   "scan",
		Short: "Hash (title, day-bucket, sorted-attendees) and surface duplicates",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			home, _ := os.UserHomeDir()
			if root == "" {
				root = filepath.Join(home, PreflightDefaultRoot)
			}
			var from time.Time
			if since != "" {
				t, err := parseAnyDate(since)
				if err != nil {
					return usageErr(err)
				}
				from = t
			}
			c, err := openGranolaCache()
			if err != nil {
				return err
			}
			groups := map[string][]map[string]any{}
			// Cache contributions.
			for id, d := range c.Documents {
				ts, _ := granola.ParseISO(d.CreatedAt)
				if !from.IsZero() && ts.Before(from) {
					continue
				}
				md := c.MeetingMetadataByID(id)
				fp := fingerprintMeeting(d.Title, ts, attendeeEmails(d, md))
				groups[fp] = append(groups[fp], map[string]any{
					"source": "cache",
					"id":     id,
					"title":  d.Title,
					"date":   ts.Format("2006-01-02"),
				})
			}
			// Filesystem contributions.
			if _, err := os.Stat(root); err == nil {
				_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
					if err != nil || info == nil || info.IsDir() {
						return nil
					}
					name := info.Name()
					if !strings.HasSuffix(name, ".md") {
						return nil
					}
					title := nameToTitle(name)
					mtime := info.ModTime()
					fp := fingerprintMeeting(title, mtime, nil)
					groups[fp] = append(groups[fp], map[string]any{
						"source": "fs",
						"path":   path,
						"title":  title,
						"date":   mtime.Format("2006-01-02"),
					})
					return nil
				})
			}
			w := cmd.OutOrStdout()
			keys := make([]string, 0, len(groups))
			for k, g := range groups {
				if len(g) >= 2 {
					keys = append(keys, k)
				}
			}
			sort.Strings(keys)
			for _, k := range keys {
				_ = emitNDJSONLine(w, map[string]any{
					"fingerprint": k,
					"count":       len(groups[k]),
					"items":       groups[k],
				})
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&root, "root", "", "Filesystem root to scan")
	cmd.Flags().StringVar(&since, "since", "", "Lower-bound date")
	return cmd
}

func attendeeEmails(d granola.Document, md *granola.MeetingMetadata) []string {
	seen := map[string]struct{}{}
	if d.People != nil {
		for _, p := range d.People.Attendees {
			if p.Email != "" {
				seen[strings.ToLower(p.Email)] = struct{}{}
			}
		}
	}
	if md != nil {
		for _, p := range md.Attendees {
			if p.Email != "" {
				seen[strings.ToLower(p.Email)] = struct{}{}
			}
		}
	}
	out := make([]string, 0, len(seen))
	for e := range seen {
		out = append(out, e)
	}
	sort.Strings(out)
	return out
}

// fingerprintMeeting returns a stable identity string for dedup detection.
// Attendees are sorted internally so callers may pass them in any order.
func fingerprintMeeting(title string, t time.Time, attendees []string) string {
	day := ""
	if !t.IsZero() {
		day = t.UTC().Format("2006-01-02")
	}
	tk := strings.ToLower(strings.TrimSpace(title))
	sorted := append([]string(nil), attendees...)
	for i := range sorted {
		sorted[i] = strings.ToLower(strings.TrimSpace(sorted[i]))
	}
	sort.Strings(sorted)
	att := strings.Join(sorted, ",")
	h := sha1.New()
	h.Write([]byte(tk))
	h.Write([]byte{0})
	h.Write([]byte(day))
	h.Write([]byte{0})
	h.Write([]byte(att))
	return hex.EncodeToString(h.Sum(nil))
}

func nameToTitle(name string) string {
	base := strings.TrimSuffix(name, ".md")
	base = strings.TrimPrefix(base, "full_")
	base = strings.TrimPrefix(base, "summary_")
	base = strings.TrimPrefix(base, "metadata_")
	return strings.ReplaceAll(base, "_", " ")
}

// Ensure fmt used.
var _ = fmt.Sprintf
