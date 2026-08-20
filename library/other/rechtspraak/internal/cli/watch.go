// Copyright 2026 markvandeven and contributors. Licensed under Apache-2.0.

package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/rechtspraak/internal/rechtspraak"
)

func newNovelWatchCmd(flags *rootFlags) *cobra.Command {
	var flagCourts []string
	var flagSubjects []string
	var flagType string
	var flagSince string
	var flagQuiet bool
	var flagFresh bool
	var flagKeyword []string
	var flagExclude []string

	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Poll for new Dutch court decisions matching a filter",
		Long: `Walk the search index over the given filter, diff against a local
cursor, and emit only ECLIs that were not seen on the previous run. Designed
for cron / agent loops:

  rechtspraak-pp-cli watch --court HR --subject belastingrecht --since 1d --quiet --json

--quiet exits silently with no output when nothing is new. Optional
--keyword / --exclude apply local narrowing in the same pass.`,
		Example: `  rechtspraak-pp-cli watch --court HR --since 7d --json
  rechtspraak-pp-cli watch --court HR --subject belastingrecht --keyword "omkering bewijslast" --quiet`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			courtIdx, err := getCourtIndex(ctx)
			if err != nil {
				return err
			}
			subjIdx, err := getSubjectIndex(ctx)
			if err != nil {
				return err
			}
			sinceT, err := parseSinceDuration(flagSince)
			if err != nil {
				return fmt.Errorf("--since: %w", err)
			}
			watchKey, watchFilter := watchCursorKey(flagCourts, flagSubjects, flagType)
			cursor := loadWatchCursor(watchKey)
			if flagFresh {
				cursor = WatchCursor{}
			}
			fromMod := cursor.LastModified
			if fromMod == "" {
				fromMod = sinceT.UTC().Format("2006-01-02T15:04:05")
			}
			params := rechtspraak.SearchParams{
				Modified: []string{fromMod},
				Max:      1000,
				Sort:     "ASC",
				Type:     flagType,
			}
			for _, c := range flagCourts {
				if uri := courtIdx.URI(c); uri != "" {
					params.Creators = append(params.Creators, uri)
				}
			}
			for _, s := range flagSubjects {
				if uri := subjIdx.URI(s); uri != "" {
					params.Subjects = append(params.Subjects, uri)
				}
			}
			http := mustHTTP()
			entries, total, err := http.Search(ctx, params)
			if err != nil {
				return err
			}
			seen := cursor.SeenSet()
			fresh := make([]rechtspraak.SearchEntry, 0, len(entries))
			var maxMod time.Time
			for _, e := range entries {
				// Advance the cursor watermark from EVERY returned entry,
				// not just fresh ones. Otherwise a polling loop where the
				// corpus is stable (all entries already in `seen`, or all
				// filtered out by --keyword/--exclude) leaves maxMod at zero
				// and the fallback below jumps the cursor to time.Now() —
				// permanently skipping any decisions whose API Modified
				// timestamp lands between the last real entry and "now".
				// The deleted skip stays at the top of the loop so a
				// withdrawn entry's timestamp never poisons the cursor.
				if e.Deleted != "" {
					continue
				}
				if e.Updated.After(maxMod) {
					maxMod = e.Updated
				}
				if seen[e.ECLI] {
					continue
				}
				if len(flagKeyword) > 0 || len(flagExclude) > 0 {
					if !narrowMatchEntry(e, flagKeyword, flagExclude) {
						continue
					}
				}
				fresh = append(fresh, e)
			}
			if maxMod.IsZero() {
				// No non-deleted entries returned at all. Hold the prior
				// cursor (don't jump to now) so the next poll re-asks for
				// the same window — server-side filtering is idempotent.
				if cursor.LastModified != "" {
					if t, perr := time.Parse("2006-01-02T15:04:05", cursor.LastModified); perr == nil {
						maxMod = t
					}
				}
				if maxMod.IsZero() {
					maxMod = time.Now().UTC()
				}
			}
			// Build the cursor we WANT to persist, but do not write it yet.
			// Persisting before output emits would silently drop fresh
			// ECLIs from future runs if anything between here and the
			// final flush goes wrong (SIGKILL, broken pipe to head/grep,
			// disk full, agent deadline). The cron use case ("ECLIs I
			// haven't reported yet") relies on this ordering: write
			// output FIRST, then commit the cursor only if every byte
			// landed cleanly.
			newCursor := WatchCursor{
				LastModified: maxMod.Format("2006-01-02T15:04:05"),
				Seen:         appendCursorECLIs(cursor.Seen, fresh, 5000),
				Filter:       watchFilter,
			}

			// Emit output. Each branch writes to stdout and then flushes
			// via the cobra OutOrStdout helper; persistence happens AFTER
			// the branch returns successfully.
			if flagQuiet && len(fresh) == 0 {
				persistWatchCursor(watchKey, newCursor)
				return nil
			}
			if shouldEmitJSON(cmd.OutOrStdout(), flags) {
				if err := writeJSONOut(cmd.OutOrStdout(), map[string]any{
					"new_count":  len(fresh),
					"total_seen": total,
					"since":      fromMod,
					"entries":    fresh,
				}); err != nil {
					return err
				}
				persistWatchCursor(watchKey, newCursor)
				return nil
			}
			if len(fresh) == 0 {
				if _, err := fmt.Fprintln(cmd.OutOrStdout(), "No new decisions."); err != nil {
					return err
				}
				persistWatchCursor(watchKey, newCursor)
				return nil
			}
			sort.SliceStable(fresh, func(i, j int) bool {
				return fresh[i].Updated.Before(fresh[j].Updated)
			})
			for _, e := range fresh {
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s  %s\n", e.ECLI, e.Title); err != nil {
					// Broken pipe (e.g. piping to head | grep) — bail
					// without persisting so the unwritten ECLIs are
					// re-emitted on the next run.
					return err
				}
			}
			persistWatchCursor(watchKey, newCursor)
			return nil
		},
	}
	cmd.Flags().StringSliceVar(&flagCourts, "court", nil, "Filter by court (afkorting, name, or PSI URI; repeatable for OR)")
	cmd.Flags().StringSliceVar(&flagSubjects, "subject", nil, "Filter by rechtsgebied (name, slug, or PSI URI; repeatable for OR)")
	cmd.Flags().StringVar(&flagType, "type", "", "Filter by document type: Uitspraak | Conclusie")
	cmd.Flags().StringVar(&flagSince, "since", "7d", "Look back this far on first run (e.g. 1h, 24h, 7d) - subsequent runs use the persisted cursor")
	cmd.Flags().BoolVar(&flagQuiet, "quiet", false, "Exit silently with no output if nothing is new (cron-friendly)")
	cmd.Flags().BoolVar(&flagFresh, "fresh", false, "Ignore the persisted cursor and start from --since again")
	cmd.Flags().StringSliceVar(&flagKeyword, "keyword", nil, "Local keyword filter (matches title + summary; repeatable for AND)")
	cmd.Flags().StringSliceVar(&flagExclude, "exclude", nil, "Local exclude filter (NOT match against title + summary)")
	return cmd
}

// WatchCursor persists watch state per filter combination so subsequent
// invocations can dedupe. Filter records the user-visible filter shape so the
// cursor file is self-describing — useful when a human inspects the state
// directory and needs to map opaque filenames back to a CLI invocation.
type WatchCursor struct {
	LastModified string         `json:"last_modified"`
	Seen         []string       `json:"seen,omitempty"`
	Filter       WatchCursorKey `json:"filter,omitempty"`
}

// WatchCursorKey is the canonical filter shape used to derive cursor filenames.
type WatchCursorKey struct {
	Courts   []string `json:"courts,omitempty"`
	Subjects []string `json:"subjects,omitempty"`
	Type     string   `json:"type,omitempty"`
}

func (c WatchCursor) SeenSet() map[string]bool {
	out := make(map[string]bool, len(c.Seen))
	for _, e := range c.Seen {
		out[e] = true
	}
	return out
}

// watchCursorKey returns (filename, filter). The filename is a SHA-256 prefix
// of the canonical filter JSON — opaque, collision-resistant, and safe on
// every OS regardless of the filter content. The human-readable filter is
// stored inside the cursor file so a human can `jq .filter <file>` to see
// what each opaque name maps to.
func watchCursorKey(courts, subjects []string, typ string) (string, WatchCursorKey) {
	// Canonicalize: sort lists and lowercase so semantically-equivalent
	// invocations share a cursor.
	sortedCourts := append([]string(nil), courts...)
	sortedSubjects := append([]string(nil), subjects...)
	sort.Strings(sortedCourts)
	sort.Strings(sortedSubjects)
	filter := WatchCursorKey{
		Courts:   sortedCourts,
		Subjects: sortedSubjects,
		Type:     typ,
	}
	canonical, _ := json.Marshal(filter)
	sum := sha256.Sum256(canonical)
	return "filter-" + hex.EncodeToString(sum[:8]), filter
}

func watchStateDir() string {
	if d := os.Getenv("RECHTSPRAAK_PP_STATE_DIR"); d != "" {
		return d
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "rechtspraak-pp-cli", "watch")
	}
	return filepath.Join(home, ".local", "state", "rechtspraak-pp-cli", "watch")
}

func loadWatchCursor(key string) WatchCursor {
	var c WatchCursor
	path := filepath.Join(watchStateDir(), key+".json")
	f, err := os.Open(path)
	if err != nil {
		return c
	}
	defer f.Close()
	_ = json.NewDecoder(f).Decode(&c)
	return c
}

func persistWatchCursor(key string, c WatchCursor) {
	dir := watchStateDir()
	_ = os.MkdirAll(dir, 0o755)
	path := filepath.Join(dir, key+".json")
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	_ = enc.Encode(c)
	_ = f.Close()
	_ = os.Rename(tmp, path)
}

func appendCursorECLIs(prev []string, fresh []rechtspraak.SearchEntry, limit int) []string {
	out := append([]string{}, prev...)
	for _, e := range fresh {
		out = append(out, e.ECLI)
	}
	if len(out) > limit {
		out = out[len(out)-limit:]
	}
	return out
}

func narrowMatchEntry(e rechtspraak.SearchEntry, keywords, excludes []string) bool {
	corpus := strings.ToLower(e.Title + "\n" + e.Summary)
	for _, kw := range keywords {
		if !strings.Contains(corpus, strings.ToLower(kw)) {
			return false
		}
	}
	for _, ex := range excludes {
		if strings.Contains(corpus, strings.ToLower(ex)) {
			return false
		}
	}
	return true
}

func nowYMD() string {
	return time.Now().UTC().Format("2006-01-02")
}

func priorYearYMD(years int) string {
	return time.Now().AddDate(-years, 0, 0).UTC().Format("2006-01-02")
}
