// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

// playbook_init.go is the per-CLI playbook auto-install path. At
// first DB open after schema migration, this reads the embed.FS from
// internal/cli/playbooks and seeds the learning_playbooks table.
//
// Designed to copy cleanly to other CLIs: replace the embed path,
// the CLI name in the SeedVersion sentinel, and the defaultDBPath
// argument. Everything else is mechanical.
//
// A sentinel row (query_family = "__seed_meta__") tracks the seed
// version. Subsequent invocations short-circuit when the sentinel
// matches. Binary upgrades that bump the SeedVersion constant
// trigger re-seed. User-authored playbooks via teach-playbook have
// different query_family keys and are untouched by re-seed.
// `playbook amend` does share family keys with seeded rows, so the
// seed loop checks each existing row and suppresses its embedded
// notes when the agent has already written notes_text — agent
// gotchas survive binary upgrades.
//
// Failures degrade gracefully: stderr warning, CLI continues without
// seeded playbooks (recall returns the empty playbook envelope, same
// as an opt-out CLI).
//
// PATCH(learn-loop-backport U9): ported from ESPN PR #851 HEAD
// 9bb0a40a (library/media-and-entertainment/espn/internal/cli/
// playbook_init.go). Import paths and CLI name adapted to
// prediction-goat. Carries the four Greptile round-4 hygiene fixes
// (use ctx properly; specific [amend marker regex; sentinel only on
// full success; install function accepts fs.FS for testability).

package cli

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/cli/playbooks"
	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/learn"
	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/store"
)

// playbookSeedSentinelFamily is the synthetic query_family used to
// track the most recent seed version. notes_text stores SeedVersion;
// absent/mismatched value triggers re-seed.
const playbookSeedSentinelFamily = "__seed_meta__"

// amendMarkerRe matches the durable signal `playbook amend` writes
// into notes_text. Greptile round 4 flagged that a bare "[amend "
// literal could appear in user-authored notes content (a quoted
// example, a code snippet), so the heuristic must anchor on the
// timestamp shape the amend command actually emits:
//
//	\n[amend YYYY-MM-DDTHH:MMZ]: ...
//
// The leading newline ensures we don't match a literal that just
// happens to appear at byte 0, and \d{4}- pins the year prefix.
var amendMarkerRe = regexp.MustCompile(`\n\[amend \d{4}-`)

// playbookInitOnce gates runPlaybookInitOnce so seeding happens at
// most once per CLI process.
var playbookInitOnce sync.Once

// runPlaybookInitOnce opens the default DB and seeds
// learning_playbooks from the embedded JSON+MD pairs in
// playbooks.FS. Idempotent: re-runs short-circuit when the sentinel
// row's seed version matches playbooks.SeedVersion. Failures
// downgrade to a stderr warning; the CLI continues without seeded
// playbooks.
func runPlaybookInitOnce(ctx context.Context) {
	playbookInitOnce.Do(func() {
		dbPath := defaultDBPath("prediction-goat-pp-cli")
		s, err := store.OpenWithContext(ctx, dbPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: prediction-goat-pp-cli: playbook init: open store: %v\n", err)
			return
		}
		defer s.Close()
		if err := installPlaybooksFromEmbed(ctx, s, playbooks.FS); err != nil {
			fmt.Fprintf(os.Stderr, "warning: prediction-goat-pp-cli: playbook init: %v\n", err)
		}
	})
}

// installPlaybooksFromEmbed walks the provided fs.FS for JSON+notes
// pairs and seeds each via store.UpsertPlaybook. The sentinel row
// tracks the current seed version; re-seeding only happens when the
// binary version bumps. Per-file parse failures log to stderr and
// are skipped. The sentinel row is only written when every per-file
// upsert succeeds — partial failures leave the sentinel untouched so
// the next install retries.
//
// Greptile round 4 hygiene fix: takes an fs.FS rather than reading
// playbooks.FS directly. Production callers pass playbooks.FS; tests
// pass an fstest.MapFS for scenario-specific fixtures. ctx is honored
// for future context-aware store APIs and currently informs the
// shape callers expect.
func installPlaybooksFromEmbed(ctx context.Context, s *store.Store, embedFS fs.FS) error {
	// Greptile round 4 hygiene fix: honor ctx for cancellation so
	// a SIGINT during install short-circuits cleanly instead of
	// completing the loop.
	if err := ctx.Err(); err != nil {
		return err
	}

	// Sentinel check: skip if seed version matches what's already installed.
	if existing, ok, err := s.GetPlaybookByFamily(playbookSeedSentinelFamily); err == nil && ok && existing.NotesText == playbooks.SeedVersion {
		return nil
	}

	entries, err := fs.ReadDir(embedFS, ".")
	if err != nil {
		return fmt.Errorf("read embed dir: %w", err)
	}
	jsonBases := make(map[string]bool, len(entries))
	notesBases := make(map[string]string, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		switch {
		case strings.HasSuffix(name, "_notes.md"):
			base := strings.TrimSuffix(name, "_notes.md")
			notesBases[base] = name
		case strings.HasSuffix(name, ".json"):
			base := strings.TrimSuffix(name, ".json")
			jsonBases[base] = true
		}
	}

	// Sort bases for deterministic seed order (matters for audit log).
	bases := make([]string, 0, len(jsonBases))
	for b := range jsonBases {
		bases = append(bases, b)
	}
	sort.Strings(bases)

	cfg := learn.DefaultPredictionGoatConfig()

	// Track whether any per-file upsert failed. The sentinel row is
	// only written when every base succeeded; this ensures that a
	// partial failure leaves the sentinel stale and the next install
	// retries (Greptile round 4 finding).
	upsertFailed := false

	for _, base := range bases {
		// Honor ctx between bases so a long install can be canceled.
		if err := ctx.Err(); err != nil {
			return err
		}
		jsonName := base + ".json"
		jsonData, rerr := fs.ReadFile(embedFS, jsonName)
		if rerr != nil {
			fmt.Fprintf(os.Stderr, "warning: prediction-goat-pp-cli: playbook init: read %s: %v\n", jsonName, rerr)
			upsertFailed = true
			continue
		}
		pb, perr := learn.ParsePlaybook(jsonData, jsonName)
		if perr != nil {
			fmt.Fprintf(os.Stderr, "warning: prediction-goat-pp-cli: playbook init: parse %s: %v\n", jsonName, perr)
			upsertFailed = true
			continue
		}
		// Derive ALL distinct query families from the example queries.
		// One playbook may cover multiple families (different phrasings
		// of the same intent normalize to different families). Seed
		// under each distinct family so any example phrasing hits the
		// same playbook+notes.
		families := make(map[string]bool)
		if len(pb.QueryFamilyExamples) > 0 {
			for _, ex := range pb.QueryFamilyExamples {
				normalized := learn.Normalize(ex, cfg)
				fam := learn.QueryFamily(normalized)
				if fam != "" {
					families[fam] = true
				}
			}
		}
		if len(families) == 0 {
			// Without query_family_examples we have no way to compute
			// a family key that `recall` would ever match — QueryFamily
			// returns a space-separated bag of non-entity tokens, and
			// the underscore-delimited filename stem can't reproduce
			// that shape. Refuse to seed under an unreachable key; the
			// authored embed must supply at least one example query.
			fmt.Fprintf(os.Stderr, "warning: prediction-goat-pp-cli: playbook init: %s has no query_family_examples; skipping (would be unreachable at recall time)\n", jsonName)
			continue
		}
		var notesText string
		if notesName, ok := notesBases[base]; ok {
			if data, nerr := fs.ReadFile(embedFS, notesName); nerr == nil {
				notesText = string(data)
			}
		}
		jsonStr, merr := learn.MarshalPlaybook(pb)
		if merr != nil {
			fmt.Fprintf(os.Stderr, "warning: prediction-goat-pp-cli: playbook init: marshal %s: %v\n", jsonName, merr)
			upsertFailed = true
			continue
		}
		// Sort families for deterministic install order.
		famList := make([]string, 0, len(families))
		for f := range families {
			famList = append(famList, f)
		}
		sort.Strings(famList)
		for _, family := range famList {
			// Two competing goals on re-seed:
			//   1. SeedVersion bumps must deliver corrected notes
			//      content to existing installs (the whole point of
			//      bumping the version).
			//   2. Notes that `playbook amend` wrote at runtime must
			//      survive binary upgrades — they encode agent-observed
			//      gotchas we don't want to lose.
			//
			// The amend command appends a "[amend YYYY-MM-DDTHH:MMZ]:"
			// marker, which is the durable signal that a row has agent
			// content. If the stored notes carry that marker, preserve
			// them; otherwise overwrite with the freshly-embedded
			// notes so the SeedVersion bump actually ships the
			// content corrections. The marker shape is matched via
			// amendMarkerRe (a leading-newline + year-prefix anchor)
			// so a literal "[amend " token in user-authored notes
			// content can't accidentally trip preservation (Greptile
			// round 4 finding).
			preserve := false
			if existing, ok, gerr := s.GetPlaybookByFamily(family); gerr == nil && ok {
				if amendMarkerRe.MatchString(existing.NotesText) {
					preserve = true
				}
			}
			if _, _, uerr := s.UpsertPlaybook(store.UpsertPlaybookInput{
				QueryFamily:           family,
				PlaybookJSON:          jsonStr,
				NotesText:             notesText,
				Source:                store.LearningSourceTaught,
				PreserveExistingNotes: preserve,
			}); uerr != nil {
				fmt.Fprintf(os.Stderr, "warning: prediction-goat-pp-cli: playbook init: upsert family=%q for %s: %v\n", family, jsonName, uerr)
				upsertFailed = true
				continue
			}
		}
	}

	// Greptile round 4 hygiene fix: write the sentinel only when
	// every per-file upsert succeeded. A partial failure must leave
	// the sentinel stale so the next CLI invocation retries.
	if upsertFailed {
		return fmt.Errorf("playbook init: one or more per-playbook upserts failed; sentinel not updated, install will retry")
	}

	// Sentinel update marks this seed version as installed.
	if _, _, err := s.UpsertPlaybook(store.UpsertPlaybookInput{
		QueryFamily: playbookSeedSentinelFamily,
		NotesText:   playbooks.SeedVersion,
		Source:      "seeded",
	}); err != nil {
		return fmt.Errorf("update sentinel: %w", err)
	}
	return nil
}
