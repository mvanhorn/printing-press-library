// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "github.com/spf13/cobra"

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newFlowDecayCmd(flags))
		addNovelCommandIfAbsent(root, newCohortCmd(flags))
		addNovelCommandIfAbsent(root, newAttributionCmd(flags))
		addNovelCommandIfAbsent(root, newDedupCmd(flags))
		addNovelCommandIfAbsent(root, newReconcileCmd(flags))
		addNovelCommandIfAbsent(root, newFlowCannibalizationCmd(flags))
		addNovelCommandIfAbsent(root, newSendFatigueCmd(flags))
		addNovelCommandIfAbsent(root, newSubjectLineAnalysisCmd(flags))
		addNovelCommandIfAbsent(root, newOptimalSendTimeCmd(flags))
		addNovelCommandIfAbsent(root, newRevenuePerEmailCmd(flags))
		addNovelCommandIfAbsent(root, newSegmentVelocityCmd(flags))
		addNovelCommandIfAbsent(root, newFlowPathAnalysisCmd(flags))
		addNovelCommandIfAbsent(root, newCampaignTimeDecayCmd(flags))
		addNovelCommandIfAbsent(root, newListQualityScoreCmd(flags))
		addNovelCommandIfAbsent(root, newContentFatigueCmd(flags))
		addNovelCommandIfAbsent(root, newPlanCmd(flags))
		addNovelCommandIfAbsent(root, newReportCmd(flags))
		addNovelCommandIfAbsent(root, newUniversalContentCmd(flags))
		for _, parent := range root.Commands() {
			if parent.Name() == "campaigns" {
				addNovelCommandIfAbsent(parent, newCampaignsDeployCmd(flags))
				addNovelCommandIfAbsent(parent, newCampaignsImageSwapCmd(flags))
				addNovelCommandIfAbsent(parent, newCampaignsABTestCmd(flags))
				addNovelCommandIfAbsent(parent, newCampaignsScheduleConflictsCmd(flags))
			}
		}
		for _, parent := range root.Commands() {
			if parent.Name() == "coupons" {
				addNovelCommandIfAbsent(parent, newCouponsCheckPoolsCmd(flags))
			}
		}
		for _, parent := range root.Commands() {
			if parent.Name() == "flows" {
				addNovelCommandIfAbsent(parent, newFlowsExportCmd(flags))
				addNovelCommandIfAbsent(parent, newFlowsCloneCmd(flags))
				addNovelCommandIfAbsent(parent, newFlowsDeployCmd(flags))
				addNovelCommandIfAbsent(parent, newFlowsPauseCmd(flags))
				addNovelCommandIfAbsent(parent, newFlowsResumeCmd(flags))
				addNovelCommandIfAbsent(parent, newFlowsHealthCmd(flags))
				addNovelCommandIfAbsent(parent, newFlowsAuditCmd(flags))
			}
		}
		for _, parent := range root.Commands() {
			if parent.Name() == "lists" {
				addNovelCommandIfAbsent(parent, newListsAuditCmd(flags))
			}
		}
		for _, parent := range root.Commands() {
			if parent.Name() == "profiles" {
				addNovelCommandIfAbsent(parent, newProfilesStatsCmd(flags))
				addNovelCommandIfAbsent(parent, newProfilesTopSpendersCmd(flags))
				addNovelCommandIfAbsent(parent, newProfilesNeverPurchasedCmd(flags))
				addNovelCommandIfAbsent(parent, newProfilesChurningCmd(flags))
				addNovelCommandIfAbsent(parent, newProfilesPruneCmd(flags))
				addNovelCommandIfAbsent(parent, newProfilesExportSuppressionsCmd(flags))
			}
		}
		for _, parent := range root.Commands() {
			if parent.Name() == "segments" {
				addNovelCommandIfAbsent(parent, newSegmentsBuildCmd(flags))
				addNovelCommandIfAbsent(parent, newSegmentsOverlapCmd(flags))
				addNovelCommandIfAbsent(parent, newSegmentsRFMCmd(flags))
			}
		}
		for _, parent := range root.Commands() {
			if parent.Name() == "tags" {
				addNovelCommandIfAbsent(parent, newTagsAuditCmd(flags))
			}
		}
		for _, parent := range root.Commands() {
			if parent.Name() == "templates" {
				addNovelCommandIfAbsent(parent, newTemplatesUpdateImageCmd(flags))
				addNovelCommandIfAbsent(parent, newTemplatesAuditCmd(flags))
			}
		}
		for _, parent := range root.Commands() {
			if parent.Name() == "auth" {
				addNovelCommandIfAbsent(parent, newAuthAccountsCmd(flags))
			}
		}
		attachPlatformClientCommands(root, flags)
		wrapPlatformCommandOutputs(root, flags)
	})
}
