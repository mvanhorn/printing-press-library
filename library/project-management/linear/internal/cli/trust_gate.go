package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/store"
)

// enforceIssueTrustMode is the mutation guard behind --trust-mode strict.
//
// The flag has always been documented as "refuses to mutate Linear issues not
// in the local pp_created table", but store.IsPPCreated had no callers, so the
// only thing strict mode actually did was demand a session tag on create. This
// helper closes that hole: every issue-mutating path calls it with the
// resolved issue UUID before the mutation goes out, and a target that is not
// an unarchived row in the pp_created ledger is refused with exit code 2.
//
// The gate fails closed. A missing or unreadable ledger means the caller
// cannot prove the issue is a fixture, so the mutation is refused rather than
// waved through.
//
// pp-cleanup is deliberately exempt: it enumerates its targets from the ledger
// itself, so re-checking membership there would be circular.
func enforceIssueTrustMode(flags *rootFlags, dbPath, issueID, issueRef string) error {
	if flags == nil || flags.trustMode != "strict" {
		return nil
	}
	if issueRef == "" {
		issueRef = issueID
	}
	if issueID == "" {
		return usageErr(fmt.Errorf("trust-mode=strict: cannot verify %q against the local pp_created ledger without a resolved issue id", issueRef))
	}
	db, err := store.Open(dbPath)
	if err != nil {
		return usageErr(fmt.Errorf("trust-mode=strict: cannot read the local pp_created ledger at %s: %w\nRun 'linear-pp-cli sync' to create the store, or drop --trust-mode strict to mutate pre-existing issues", dbPath, err))
	}
	defer db.Close()

	created, err := db.IsPPCreated(issueID)
	if err != nil {
		return usageErr(fmt.Errorf("trust-mode=strict: reading the local pp_created ledger failed: %w", err))
	}
	if !created {
		return usageErr(fmt.Errorf("trust-mode=strict: %s is not in the local pp_created ledger, so this CLI did not create it and refuses to mutate it.\nRun 'linear-pp-cli pp-test list' to see the fixtures this CLI owns, or drop --trust-mode strict to mutate pre-existing workspace issues", issueRef))
	}
	return nil
}
