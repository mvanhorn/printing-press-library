package store

import (
	"time"

	"github.com/mvanhorn/printing-press-library/library/internal/intelcli"
)

func Fixture(profile string) DataSet {
	now := time.Now().UTC()
	start, end := intelcli.DefaultDateRange("", "")
	return DataSet{
		Profile:    profile,
		SyncedAt:   now,
		Source:     "embedded-paid-ads-fixture",
		Provenance: DataProvenance{SchemaVersion: "ads-intel.provenance/v1", DateRange: DateRange{StartDate: start, EndDate: end}, SourceCommandVersions: map[string]string{"ads-intel-pp-cli": "0.1.0-private"}, InputHashes: map[string]string{"embedded_fixture": "sha256:embedded"}},
		Account:    AccountStatus{AccountID: "demo-account", Name: "Demo Ads Account", Status: "active", Currency: "USD"},
		Campaigns: []Campaign{
			{ID: "g-1", Name: "Brand Search", Platform: "google", Status: "enabled", Spend: 420, Conversions: 42, Revenue: 8400, Clicks: 800, Impressions: 12000, CTR: .066, ROAS: 20},
			{ID: "g-2", Name: "Nonbrand Prospecting", Platform: "google", Status: "enabled", Spend: 680, Conversions: 3, Revenue: 420, Clicks: 510, Impressions: 22000, CTR: .023, ROAS: .62},
			{ID: "m-1", Name: "Meta Broad Creative Test", Platform: "meta", Status: "active", Spend: 350, Conversions: 8, Revenue: 1200, Clicks: 290, Impressions: 60000, Frequency: 3.4, CTR: .0048, ROAS: 3.4},
			{ID: "a-1", Name: "Amazon Sponsored Products", Platform: "amazon", Status: "enabled", Spend: 260, Conversions: 14, Revenue: 2100, Clicks: 240, Impressions: 18000, CTR: .013, ROAS: 8.1},
			{ID: "m-2", Name: "Meta Learning Campaign", Platform: "meta", Status: "active", Spend: 80, Conversions: 1, Revenue: 60, Clicks: 75, Impressions: 9000, Frequency: 1.4, CTR: .008, ROAS: .75, LearningPhase: true},
		},
		SearchTerms: []SearchTerm{
			{Platform: "google", CampaignID: "g-2", CampaignName: "Nonbrand Prospecting", AdGroupID: "ag-22", AdGroupName: "Prospecting Terms", Term: "free hiking boots", Spend: 18.50, Conversions: 0, Clicks: 31},
			{Platform: "amazon", CampaignID: "a-1", CampaignName: "Amazon Sponsored Products", Term: "cheap boot repair", Spend: 12.25, Conversions: 0, Clicks: 18},
		},
		Keywords: []Keyword{
			{Platform: "google", CampaignID: "g-1", Text: "acme boots", MatchType: "EXACT", Bidding: "TARGET_ROAS", Clicks: 300, Conversions: 30, NegativeList: "brand-safe"},
			{Platform: "google", CampaignID: "g-2", Text: "hiking boots", MatchType: "BROAD", Bidding: "TARGET_CPA", Clicks: 140, Conversions: 0},
			{Platform: "google", CampaignID: "g-2", Text: "+trail +boots", MatchType: "BROAD", Bidding: "MANUAL_CPC", Clicks: 160, Conversions: 0},
			{Platform: "google", CampaignID: "g-2", Text: "waterproof shoes", MatchType: "BROAD", Bidding: "TARGET_CPA", Clicks: 130, Conversions: 0},
			{Platform: "google", CampaignID: "g-2", Text: "best winter boots", MatchType: "BROAD", Bidding: "TARGET_CPA", Clicks: 125, Conversions: 0},
			{Platform: "google", CampaignID: "g-2", Text: "free boot coupons", MatchType: "BROAD", Bidding: "TARGET_CPA", Clicks: 111, Conversions: 0},
		},
		NegativeLists: []NegativeList{{Platform: "google", ID: "nl-1", Name: "brand-safe", Shared: true, Campaigns: []string{"g-1", "g-2"}}},
	}
}
