// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

// Local-store helpers for the hand-written commands: read-only access that
// never creates the store (dry-run and auto resolution), id parsing, and the
// local-first data-source rule used by check, screen, and stats.

package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/uber-jobs/internal/store"
	"github.com/mvanhorn/printing-press-library/library/productivity/uber-jobs/internal/uberjobs"
)

var postingIDPattern = regexp.MustCompile(`^[0-9]{1,12}$`)

// validPostingID reports whether s is a requisition id: digits only.
func validPostingID(s string) bool { return postingIDPattern.MatchString(s) }

// parsePostingIDs reads ids from args, or from stdin when the only arg is
// "-". Ids may be separated by whitespace or commas; duplicates keep their
// first position. Any token that is not an id is a usage error.
func parsePostingIDs(args []string, stdin io.Reader) ([]string, error) {
	var tokens []string
	if len(args) == 1 && args[0] == "-" {
		sc := bufio.NewScanner(stdin)
		sc.Buffer(make([]byte, 64*1024), 4<<20)
		for sc.Scan() {
			tokens = append(tokens, strings.FieldsFunc(sc.Text(), isIDSeparator)...)
		}
		if err := sc.Err(); err != nil {
			return nil, fmt.Errorf("reading ids from stdin: %w", err)
		}
	} else {
		for _, a := range args {
			tokens = append(tokens, strings.FieldsFunc(a, isIDSeparator)...)
		}
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(tokens))
	for _, t := range tokens {
		if !validPostingID(t) {
			return nil, usageErr(fmt.Errorf("%q is not a posting id: ids are the digits in a jobs.uber.com/en/jobs/<id>/ URL", t))
		}
		if !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	return out, nil
}

func isIDSeparator(r rune) bool {
	return r == ',' || r == ' ' || r == '\t' || r == '\n' || r == '\r'
}

// storeFileExists reports whether the local store file exists, so read-only
// paths never create it.
func storeFileExists(dbPath string) bool {
	_, err := os.Stat(uberDBPath(dbPath))
	return err == nil
}

// isMissingTable reports SQLite's error for a store our commands never
// migrated (the learn loop can create the file first).
func isMissingTable(err error) bool {
	return err != nil && strings.Contains(err.Error(), "no such table")
}

// withStoreRO runs fn against a read-only handle on the store. A missing
// file runs nothing and returns false.
func withStoreRO(ctx context.Context, dbPath string, fn func(*store.Store) error) (bool, error) {
	if !storeFileExists(dbPath) {
		return false, nil
	}
	s, err := store.OpenReadOnlyContext(ctx, uberDBPath(dbPath))
	if err != nil {
		return false, fmt.Errorf("opening local store: %w", err)
	}
	defer s.Close()
	return true, fn(s)
}

// lastFullSyncRO returns the latest complete, uncapped, whole-corpus sync
// without creating or migrating the store. No file or no tables means none.
func lastFullSyncRO(ctx context.Context, dbPath string) (*uberjobs.SyncRun, error) {
	var run *uberjobs.SyncRun
	_, err := withStoreRO(ctx, dbPath, func(s *store.Store) error {
		r, err := uberjobs.LastFullSync(ctx, s.DB())
		if isMissingTable(err) {
			return nil
		}
		run = r
		return err
	})
	return run, err
}

// savedSearchRO reads one saved search without creating the store.
func savedSearchRO(ctx context.Context, dbPath, name string) (*uberjobs.SavedSearch, error) {
	var saved *uberjobs.SavedSearch
	_, err := withStoreRO(ctx, dbPath, func(s *store.Store) error {
		got, err := uberjobs.GetSearch(ctx, s.DB(), name)
		if isMissingTable(err) {
			return nil
		}
		saved = got
		return err
	})
	return saved, err
}

// resolveLocalFirst turns auto into local when the store holds a complete
// full sync no older than maxAge (any age when maxAge is 0), and into live
// otherwise. It never creates the store, so --dry-run can call it.
func resolveLocalFirst(ctx context.Context, ds, dbPath string, maxAge time.Duration, now time.Time) (string, *uberjobs.SyncRun, error) {
	last, err := lastFullSyncRO(ctx, dbPath)
	if err != nil {
		return "", nil, err
	}
	if ds != "auto" {
		return ds, last, nil
	}
	if last != nil && (maxAge <= 0 || syncAge(last, now) <= maxAge) {
		return "local", last, nil
	}
	return "live", last, nil
}

// syncAge is how long ago a sync finished; an unparseable stamp is treated
// as infinitely old.
func syncAge(r *uberjobs.SyncRun, now time.Time) time.Duration {
	t, err := time.Parse(time.RFC3339, r.FinishedAt)
	if err != nil {
		return time.Duration(1<<63 - 1)
	}
	return now.Sub(t)
}

// localFirstNote explains which source auto picked, for meta.note.
func localFirstNote(ds string, last *uberjobs.SyncRun) string {
	switch {
	case ds == "local" && last != nil:
		return "from the local store, last complete sync " + last.FinishedAt
	case ds == "local":
		return "the local store has no complete sync yet; run: uber-jobs-pp-cli sync"
	default:
		return ""
	}
}
