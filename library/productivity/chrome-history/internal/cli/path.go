package cli

import (
	"errors"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/chrome-history/internal/store"
)

func snapshotPath() (string, error) {
	return store.SnapshotPath()
}

func openActiveStore() (*store.Store, bool, error) {
	st, isArchive, err := store.OpenActiveStore()
	if err != nil {
		if errors.Is(err, store.ErrNoSnapshot) {
			return nil, false, ErrNoSnapshot
		}
		return nil, false, err
	}
	return st, isArchive, nil
}

func openSnapshotStore() (*store.Store, error) {
	path, err := snapshotPath()
	if err != nil {
		return nil, err
	}
	st, err := store.OpenExisting(path)
	if err != nil {
		if errors.Is(err, store.ErrNoSnapshot) {
			return nil, ErrNoSnapshot
		}
		return nil, err
	}
	return st, nil
}

func openCoreHistoryStore(device string) (*store.Store, bool, error) {
	d := strings.TrimSpace(strings.ToLower(device))
	if d != "" && d != "all" {
		st, err := openSnapshotStore()
		return st, false, err
	}
	return openActiveStore()
}
