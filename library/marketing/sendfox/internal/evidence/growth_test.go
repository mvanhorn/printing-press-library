package evidence

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func completeResource(items []Row, observedAt string) Resource {
	complete := true
	total := len(items)
	return Resource{Items: items, Source: "synthetic", ObservedAt: observedAt, Complete: &complete, Total: &total, Pages: 1, ScopesComplete: true}
}

func TestGrowthReportSnapshotComparisonAndMetricProvenance(t *testing.T) {
	previousAt := "2026-08-01T00:00:00Z"
	currentAt := "2026-09-01T00:00:00Z"
	previous := Snapshot{SchemaVersion: 1, Resources: map[string]Resource{
		"contacts":     completeResource([]Row{{"id": 1}, {"id": 2}}, previousAt),
		"unsubscribed": completeResource([]Row{}, previousAt),
		"lists":        completeResource([]Row{{"id": 10, "name": "Readers"}}, previousAt),
		"memberships":  completeResource([]Row{{"contact_id": 1, "list_id": 10}, {"contact_id": 2, "list_id": 10}}, previousAt),
	}}
	current := Snapshot{SchemaVersion: 1, Previous: &previous, Resources: map[string]Resource{
		"contacts": completeResource([]Row{
			{"id": 2, "created_at": "2026-07-01T00:00:00Z"},
			{"id": 3, "created_at": "2026-08-20T00:00:00Z"},
		}, currentAt),
		"unsubscribed": completeResource([]Row{{"id": 1, "unsubscribed_at": "2026-08-25T00:00:00Z"}}, currentAt),
		"lists":        completeResource([]Row{{"id": 10, "name": "Readers"}}, currentAt),
		"memberships":  completeResource([]Row{{"contact_id": 2, "list_id": 10}, {"contact_id": 3, "list_id": 10}}, currentAt),
	}}

	report := GrowthReport(current, time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), 0, 30*24*time.Hour)
	if report.Data["new_contacts"].(Row)["value"] != 1 || report.Data["unsubscribes"].(Row)["value"] != 1 || report.Data["observed_net_growth"].(Row)["value"] != 0 {
		t.Fatal(report.Data)
	}
	comparison := report.Data["snapshot_comparison"].(Row)
	lists := comparison["lists"].([]Row)
	if len(lists) != 1 {
		t.Fatal(lists)
	}
	list := lists[0]
	if list["joins"].(Row)["value"] != 1 || list["leaves"].(Row)["value"] != 1 || list["retention_rate"].(Row)["value"] != float64(50) {
		t.Fatal(list)
	}
	for _, key := range []string{"joins", "leaves", "retention_rate", "net_growth"} {
		metric := list[key].(Row)
		for _, field := range []string{"formula", "window", "numerator", "denominator", "evidence_grade", "complete", "warnings"} {
			if _, ok := metric[field]; !ok {
				t.Fatalf("%s metric missing %s: %#v", key, field, metric)
			}
		}
	}
}

func TestGrowthReportMissingTimestampsAreUnknownNotZero(t *testing.T) {
	s := Snapshot{SchemaVersion: 1, Resources: map[string]Resource{
		"contacts":     completeResource([]Row{{"id": 1}}, "2026-09-01T00:00:00Z"),
		"unsubscribed": completeResource([]Row{{"id": 2}}, "2026-09-01T00:00:00Z"),
		"lists":        completeResource([]Row{}, "2026-09-01T00:00:00Z"),
		"memberships":  completeResource([]Row{}, "2026-09-01T00:00:00Z"),
	}}
	report := GrowthReport(s, time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), 0, 30*24*time.Hour)
	if report.Data["new_contacts"].(Row)["value"] != nil || report.Data["unsubscribes"].(Row)["value"] != nil || report.Data["observed_net_growth"].(Row)["value"] != nil {
		t.Fatal(report.Data)
	}
}

func TestWriteSnapshotHistoryIsChecksummedAtomicAndPrivate(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "history")
	s := Snapshot{SchemaVersion: 1, Resources: map[string]Resource{}}
	artifact, err := WriteSnapshotHistory(dir, s, time.Date(2026, 9, 21, 12, 30, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(artifact.Path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("snapshot mode = %o", info.Mode().Perm())
	}
	dirInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if dirInfo.Mode().Perm() != 0700 {
		t.Fatalf("history mode = %o", dirInfo.Mode().Perm())
	}
	payload, err := os.ReadFile(artifact.Path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(payload)
	if artifact.SHA256 != hex.EncodeToString(sum[:]) || artifact.Bytes != len(payload) {
		t.Fatalf("artifact mismatch: %#v", artifact)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("partial files left behind: %v", entries)
	}
}

func TestAudienceHealthEngagementCohortsDistinguishNeverFromUnknown(t *testing.T) {
	asOf := "2026-09-21T00:00:00Z"
	s := Snapshot{SchemaVersion: 1, Resources: map[string]Resource{
		"contacts": completeResource([]Row{
			{"id": 1, "email": "active@example.com", "last_opened_at": "2026-09-15T00:00:00Z"},
			{"id": 2, "email": "never@example.com"},
		}, asOf),
		"unsubscribed":   completeResource([]Row{}, asOf),
		"lists":          completeResource([]Row{}, asOf),
		"memberships":    completeResource([]Row{}, asOf),
		"tags":           completeResource([]Row{}, asOf),
		"contact_tags":   completeResource([]Row{}, asOf),
		"contact_fields": completeResource([]Row{}, asOf),
		"activity":       completeResource([]Row{}, asOf),
	}}
	report := AudienceHealth(s, time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC), 0)
	cohorts := report.Data["engagement_cohorts"].(Row)
	values := map[string]Row{}
	for _, row := range cohorts["cohorts"].([]Row) {
		values[Text(row["cohort"])] = row
	}
	if values["active_30d"]["count"] != 1 || values["never_engaged"]["count"] != 1 || values["unknown"]["count"] != 0 {
		t.Fatal(values)
	}
	activity := s.Resources["activity"]
	incomplete := false
	activity.Complete = &incomplete
	s.Resources["activity"] = activity
	report = AudienceHealth(s, time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC), 0)
	cohorts = report.Data["engagement_cohorts"].(Row)
	values = map[string]Row{}
	for _, row := range cohorts["cohorts"].([]Row) {
		values[Text(row["cohort"])] = row
	}
	if values["never_engaged"]["count"] != nil || values["unknown"]["count"] != 1 {
		t.Fatal(values)
	}
}

func TestCampaignReviewUsesRecipientWeightedBenchmarks(t *testing.T) {
	s := fixture(t)
	stats := s.Resources["campaign_stats"]
	stats.Items = []Row{
		{"campaign_id": 42, "sent_count": 100, "unique_open_count": 50, "unique_click_count": 10},
		{"campaign_id": 43, "sent_count": 10, "unique_open_count": 10, "unique_click_count": 5},
	}
	totalStats := len(stats.Items)
	stats.Total = &totalStats
	s.Resources["campaign_stats"] = stats
	campaigns := s.Resources["campaigns"]
	campaigns.Items = []Row{
		{"id": 42, "status": "sent", "sent_at": "2026-08-01T00:00:00Z"},
		{"id": 43, "status": "sent", "sent_at": "2026-09-01T00:00:00Z"},
	}
	totalCampaigns := len(campaigns.Items)
	campaigns.Total = &totalCampaigns
	s.Resources["campaigns"] = campaigns

	report := CampaignReview(s, "42", "non_openers", time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC), 0)
	benchmarks := report.Data["benchmarks"].(Row)
	openRate := benchmarks["open_rate"].(Row)
	weighted := openRate["recipient_weighted"].(Row)
	medianRate := openRate["campaign_median"].(Row)
	trend := openRate["trend"].(Row)
	if got := weighted["value"].(float64); got < 54.54 || got > 54.55 {
		t.Fatalf("weighted open rate = %v", got)
	}
	if medianRate["value"] != float64(75) {
		t.Fatalf("median = %#v", medianRate)
	}
	if trend["value"] != float64(50) {
		t.Fatalf("trend = %#v", trend)
	}
}
