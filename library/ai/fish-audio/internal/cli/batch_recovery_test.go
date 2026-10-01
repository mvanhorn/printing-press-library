package cli

import (
	"errors"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/ai/fish-audio/internal/fishaudio"
)

func TestAccumulateBatchResultPreservesPartialRecoveryManifest(t *testing.T) {
	unitOne := batchUnit{path: "one.mp3", req: fishaudio.RenderRequest{Text: "hello"}}
	unitTwo := batchUnit{path: "two.mp3", req: fishaudio.RenderRequest{Text: "hello"}}
	result := batchResult{
		job:                batchJob{units: []batchUnit{unitOne, unitTwo}},
		err:                errors.New("second output failed"),
		synthesisCompleted: true,
		costUSD:            0.25,
		manifests: []renderManifest{{
			ID: 41, File: "one.mp3", BytesOut: 12, CostUSD: 0.25,
		}},
	}
	summary := batchSummary{Renders: []renderManifest{}, Failed: []batchFailure{}}
	completed := accumulateBatchResult(&summary, result)
	if !completed["one.mp3"] || completed["two.mp3"] {
		t.Fatalf("completed paths = %v", completed)
	}
	if summary.Count != 1 || summary.Files != 1 || summary.BytesOut != 12 || len(summary.Renders) != 1 || summary.Renders[0].ID != 41 {
		t.Fatalf("partial recovery summary = %+v", summary)
	}
}

func TestAccumulateBatchResultAccountsForSynthesisBeforePersistenceFailure(t *testing.T) {
	unit := batchUnit{path: "unwritten.mp3", req: fishaudio.RenderRequest{Text: "provider returned this audio"}}
	result := batchResult{
		job:                batchJob{units: []batchUnit{unit}},
		synthesisCompleted: true,
		costUSD:            0.25,
		paidEquivUSD:       0.50,
		err:                errors.New("first output write failed"),
	}
	summary := batchSummary{Renders: []renderManifest{}, Failed: []batchFailure{}}
	completed := accumulateBatchResult(&summary, result)
	if len(completed) != 0 || summary.Files != 0 || len(summary.Renders) != 0 || summary.BytesOut != 0 {
		t.Fatalf("persistence totals = completed %v, summary %+v; want no durable output", completed, summary)
	}
	if summary.Count != 1 || summary.BytesIn != unit.req.BytesIn64() || summary.CostUSD != 0.25 || summary.CostUSDPaidEquiv != 0.50 {
		t.Fatalf("provider accounting = %+v, want one completed billed synthesis", summary)
	}
}
