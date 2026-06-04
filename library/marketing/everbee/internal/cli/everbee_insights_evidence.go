package cli

import (
	"context"

	"github.com/mvanhorn/printing-press-library/library/marketing/everbee/internal/research"

	"github.com/spf13/cobra"
)

type researchEnvelopeBuilder func(research.ResearchPlan, research.Snapshot, []research.Snapshot) research.ResponseEnvelope

func runResearchEnvelopeCommand(cmd *cobra.Command, flags *rootFlags, scope research.ResearchScope, opts researchOptions, build researchEnvelopeBuilder) error {
	ctx := cmd.Context()
	result, snapshots, err := resolveResearchForCommand(ctx, flags, scope, opts)
	if err != nil {
		return err
	}
	return printJSONFiltered(cmd.OutOrStdout(), build(result.Plan, result.Snapshot, snapshotsWithResult(snapshots, result.Snapshot)), flags)
}

func resolveResearchForCommand(ctx context.Context, flags *rootFlags, scope research.ResearchScope, opts researchOptions) (researchResult, []research.Snapshot, error) {
	db, err := openResearchDB(ctx, opts.noRefresh)
	if err != nil {
		return researchResult{}, nil, err
	}
	if db != nil {
		defer db.Close()
	}
	snapshotStore := researchStoreFromDB(db)
	var snapshots []research.Snapshot
	if snapshotStore != nil {
		snapshots, err = snapshotStore.List(ctx, scope, 5)
		if err != nil {
			return researchResult{}, nil, err
		}
	}

	runtime := researchRuntime{fetcher: liveResearchFetcher{flags: flags, snapshotStore: snapshotStore, maxAge: opts.maxAge}}
	if opts.noRefresh {
		runtime.fetcher = nil
	}
	result, err := runtime.resolve(ctx, scope, opts, snapshots)
	if err != nil {
		return researchResult{}, nil, err
	}
	return result, snapshots, nil
}

func evidenceFromSnapshot(plan research.ResearchPlan, snapshot research.Snapshot) ([]research.EvidenceRecord, research.ResearchPlan) {
	evidence, warnings := opportunityEvidence(snapshot)
	return evidence, appendResearchWarnings(plan, warnings)
}

func snapshotsWithResult(snapshots []research.Snapshot, snapshot research.Snapshot) []research.Snapshot {
	if len(snapshot.Evidence) == 0 && len(snapshot.RawRecords) == 0 {
		return snapshots
	}
	for _, existing := range snapshots {
		if existing.ID != 0 && existing.ID == snapshot.ID {
			return snapshots
		}
		if existing.FetchedAt.Equal(snapshot.FetchedAt) && existing.Scope == snapshot.Scope {
			return snapshots
		}
	}
	return append(snapshots, snapshot)
}
