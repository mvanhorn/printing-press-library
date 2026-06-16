package store

import "time"

func Fixture(profile string) DataSet {
	now := time.Now().UTC()
	seller := SourceEvidence{Present: true, Source: "embedded-fixture", ChildCLICommand: "amazon-seller-pp-cli <read-only command> --agent", SyncedAt: now}
	ads := SourceEvidence{Present: true, Source: "embedded-fixture", ChildCLICommand: "amazon-ads-pp-cli <read-only command> --agent", SyncedAt: now}
	brand := SourceEvidence{Present: true, Source: "embedded-fixture", ChildCLICommand: "amazon-seller-pp-cli brand-analytics <report> --agent", SyncedAt: now}
	listings := SourceEvidence{Present: true, Source: "embedded-fixture", ChildCLICommand: "amazon-seller-pp-cli listing-intel health-audit --agent", SyncedAt: now}
	vendor := SourceEvidence{Present: true, Source: "local-import", ImportedFrom: "embedded vendor fixture", SyncedAt: now}
	local := SourceEvidence{Present: true, Source: "embedded-fixture", SyncedAt: now}

	skus := []SKU{
		sku("DAYPACK-BLK", "B00DAYPACK", "Trail Daypack Black", "ATVPDKIKX0DER", "TrailCo", "Backpacks", 18420, 372, 2420, .154, .031, 42, 12.40, 14, 220, false, 18.80, 2460, 6920, .355, .134, .32, 86, false, nil, 0, seller, ads, listings),
		sku("WATERBOTTLE-32", "B00BOTTLE", "Insulated Bottle 32oz", "ATVPDKIKX0DER", "TrailCo", "Hydration", 22160, 554, 3840, .144, .041, 76, 8.20, 23, 148, false, 14.10, 3180, 8120, .392, .143, .30, 78, false, []string{"main image could be stronger"}, 0, seller, ads, listings),
		sku("YOGA-MAT-GRN", "B00YOGAMAT", "Eco Yoga Mat Green", "ATVPDKIKX0DER", "FlexHome", "Fitness", 16340, 430, 3010, .143, .025, 9, 6.30, 14, 64, false, 15.90, 2840, 7720, .368, .174, .34, 91, false, nil, 0, seller, ads, listings),
		sku("CANDLE-LAV-3PK", "B00CANDLE", "Lavender Candle 3 Pack", "ATVPDKIKX0DER", "HomeGlow", "Home", 12880, 322, 2510, .128, .096, 580, 81.10, 14, 236, false, 7.40, 1920, 4280, .449, .149, .28, 74, false, []string{"return comments mention damaged packaging"}, 380, seller, ads, listings),
		sku("PET-BRUSH-PRO", "B00PETBRUSH", "Pro Pet Grooming Brush", "ATVPDKIKX0DER", "PetLane", "Pet Supplies", 9750, 390, 2160, .181, .035, 640, 71.30, 21, 204, false, 11.80, 220, 1660, .133, .023, .36, 82, false, nil, 0, seller, ads, listings),
		sku("AIR-FILTER-HEPA", "B00HEPAFLT", "HEPA Replacement Filter", "ATVPDKIKX0DER", "HomeGlow", "Home", 19900, 398, 2620, .152, .028, 118, 24.50, 30, 176, false, 19.40, 1740, 5660, .307, .087, .34, 88, false, nil, 0, seller, ads, listings),
		sku("DESK-LAMP-WHT", "B00DESKLMP", "Dimmable Desk Lamp White", "ATVPDKIKX0DER", "BrightDesk", "Office", 14350, 205, 2380, .086, .049, 0, 0, 28, 18, true, 6.20, 2480, 3220, .770, .173, .24, 67, false, []string{"stranded inventory", "weak bullet copy"}, 0, seller, ads, listings),
		sku("PHONE-STAND-AL", "B00PHONEST", "Aluminum Phone Stand", "ATVPDKIKX0DER", "BrightDesk", "Office", 11860, 593, 3550, .167, .021, 48, 4.90, 10, 112, false, 9.90, 2380, 4520, .527, .201, .31, 71, false, []string{"A+ content missing"}, 0, seller, ads, listings),
		sku("FIXTURE-SKU", "B00FIXTURE", "Launch Ready Compression Socks", "ATVPDKIKX0DER", "FlexHome", "Fitness", 0, 0, 0, 0, 0, 250, 999, 21, 0, false, 0, 0, 0, 0, 0, .25, 62, false, []string{"launch listing has only four images", "keyword file missing broad match plan"}, 0, seller, ads, listings),
		sku("SUPPRESSED-TEE", "B00SUPTEE", "Graphic Training Tee", "ATVPDKIKX0DER", "FlexHome", "Apparel", 8140, 271, 1880, .144, .032, 164, 54.50, 21, 101, false, 4.70, 980, 1880, .521, .120, .27, 38, true, []string{"suppressed listing", "missing size chart", "title too short"}, 0, seller, ads, listings),
	}

	return DataSet{
		Profile:  profile,
		Source:   "embedded-fixture",
		SyncedAt: now,
		SKUs:     skus,
		Campaigns: []Campaign{
			campaign("cmp-daypack", "Daypack core exact", "DAYPACK-BLK", "B00DAYPACK", 1540, 4680, 36, 1120, 64000, .329, "healthy", ads),
			campaign("cmp-yoga", "Yoga mat launch defense", "YOGA-MAT-GRN", "B00YOGAMAT", 2260, 6400, 44, 1380, 87000, .353, "healthy", ads),
			campaign("cmp-desk-lamp", "Desk lamp auto", "DESK-LAMP-WHT", "B00DESKLMP", 2120, 1420, 11, 920, 53000, 1.493, "spending_while_out_of_stock", ads),
			campaign("cmp-phone-stand", "Phone stand discovery", "PHONE-STAND-AL", "B00PHONEST", 1860, 2940, 22, 1040, 62000, .633, "above_break_even", ads),
			campaign("cmp-pet-brush", "Pet brush branded defense", "PET-BRUSH-PRO", "B00PETBRUSH", 220, 1660, 18, 260, 28000, .133, "underfunded_winner", ads),
			campaign("cmp-suppressed", "Training tee broad", "SUPPRESSED-TEE", "B00SUPTEE", 980, 1880, 15, 610, 47000, .521, "listing_defect", ads),
		},
		SearchTerms: []SearchTerm{
			search("hiking daypack", "DAYPACK-BLK", "B00DAYPACK", 320, 1740, 17, 260, 15300, 5, .18, "promote_exact", ads, brand),
			search("cheap desk lamp", "DESK-LAMP-WHT", "B00DESKLMP", 410, 0, 0, 210, 12200, 24, .01, "add_negative", ads, brand),
			search("yoga mat non slip", "YOGA-MAT-GRN", "B00YOGAMAT", 720, 2880, 21, 410, 22400, 8, .12, "defend_rank", ads, brand),
			search("pet grooming brush", "PET-BRUSH-PRO", "B00PETBRUSH", 94, 790, 9, 140, 8900, 3, .22, "increase_budget", ads, brand),
			search("phone stand for desk", "PHONE-STAND-AL", "B00PHONEST", 360, 260, 3, 190, 9900, 2, .19, "reduce_paid", ads, brand),
			search("training tee graphic", "SUPPRESSED-TEE", "B00SUPTEE", 260, 190, 2, 120, 7600, 31, .04, "pause_until_listing_fixed", ads, brand),
		},
		Listings: []ListingHealth{
			listing("DESK-LAMP-WHT", "B00DESKLMP", "Dimmable Desk Lamp White", 67, false, []string{"stranded inventory", "weak bullet copy"}, 2380, .086, 2480, listings),
			listing("PHONE-STAND-AL", "B00PHONEST", "Aluminum Phone Stand", 71, false, []string{"A+ content missing"}, 3550, .167, 2380, listings),
			listing("SUPPRESSED-TEE", "B00SUPTEE", "Graphic Training Tee", 38, true, []string{"suppressed listing", "missing size chart"}, 1880, .144, 980, listings),
			listing("FIXTURE-SKU", "B00FIXTURE", "Launch Ready Compression Socks", 62, false, []string{"only four images", "keyword plan incomplete"}, 0, 0, 0, listings),
		},
		PurchaseOrders: []PurchaseOrder{
			{POID: "PO-1001", SKU: "YOGA-MAT-GRN", ASIN: "B00YOGAMAT", Units: 900, UnitCost: 8.10, ExpectedShipDate: "2026-06-24", ExpectedReceiveDate: "2026-07-05", Status: "planned", Source: MetricSources{LocalImport: local, VendorFiles: vendor}},
			{POID: "PO-1002", SKU: "DESK-LAMP-WHT", ASIN: "B00DESKLMP", Units: 500, UnitCost: 23.50, ExpectedShipDate: "2026-06-18", ExpectedReceiveDate: "2026-06-29", Status: "ship_window_at_risk", Source: MetricSources{LocalImport: local, VendorFiles: vendor}},
		},
		VendorDeductions: []VendorDeduction{
			{ID: "DED-44", Type: "chargeback", SKU: "CANDLE-LAV-3PK", ASIN: "B00CANDLE", Amount: 740, Reason: "carton damage", DisputeBy: "2026-06-27", Confidence: .82, Source: MetricSources{VendorFiles: vendor}},
			{ID: "DED-45", Type: "shortage", SKU: "AIR-FILTER-HEPA", ASIN: "B00HEPAFLT", Amount: 1180, Reason: "quantity variance", DisputeBy: "2026-06-30", Confidence: .71, Source: MetricSources{VendorFiles: vendor}},
		},
		BundleSignals: []BundleSignal{
			{PrimaryASIN: "B00DAYPACK", SecondaryASIN: "B00BOTTLE", PrimarySKU: "DAYPACK-BLK", SecondarySKU: "WATERBOTTLE-32", Confidence: .87, CombinedMargin: .31, InventoryFeasible: true, SuggestedOffer: "Trail daypack hydration starter bundle", Source: MetricSources{BrandAnalytics: brand}},
			{PrimaryASIN: "B00DESKLMP", SecondaryASIN: "B00PHONEST", PrimarySKU: "DESK-LAMP-WHT", SecondarySKU: "PHONE-STAND-AL", Confidence: .74, CombinedMargin: .18, InventoryFeasible: false, SuggestedOffer: "Desk setup bundle", Source: MetricSources{BrandAnalytics: brand}},
		},
		LaunchPlans: []LaunchPlan{
			{SKU: "FIXTURE-SKU", ASIN: "B00FIXTURE", TargetACOS: .25, LaunchBudget: 500, InventoryUnits: 250, COGS: 12.50, Keywords: []string{"compression socks", "running compression socks"}, ListingScore: 62, Source: MetricSources{LocalImport: local}},
		},
		Account: AccountHealth{Score: 84, AtRiskCount: 3, ReturnSpikeCount: 1, ReimbursementFlags: 2, SettlementGap: 430, Source: MetricSources{Seller: seller}},
	}
}

func sku(id, asin, title, marketplace, brand, category string, revenue float64, units, sessions int, cvr, returns float64, fba int, cover float64, lead, aging int, stranded bool, profit float64, adSpend, adSales, acos, tacos, breakEven, listingScore float64, suppressed bool, defects []string, reimbursement float64, seller, ads, listings SourceEvidence) SKU {
	if profit > 0 && profit < revenue*.05 {
		profit = profit * float64(units)
	}
	return SKU{SKU: id, ASIN: asin, Title: title, MarketplaceID: marketplace, Brand: brand, Category: category, Revenue: revenue, UnitsSold: units, Sessions: sessions, ConversionRate: cvr, ReturnRate: returns, FBAAvailable: fba, DaysOfCover: cover, LeadTimeDays: lead, AgingDays: aging, Stranded: stranded, COGS: revenue * .32, ReferralFees: revenue * .15, FBAFees: revenue * .12, StorageFees: float64(maxInt(0, aging-180)) * .8, Reimbursements: reimbursement, Profit: profit, ContributionMargin: margin(profit, revenue), AdSpend: adSpend, AdSales: adSales, ACOS: acos, TACOS: tacos, BreakEvenACOS: breakEven, ListingScore: listingScore, Suppressed: suppressed, Defects: defects, ReimbursementDue: reimbursement, Source: MetricSources{Seller: seller, Ads: ads, Listings: listings}}
}

func campaign(id, name, skuID, asin string, spend, sales float64, orders, clicks, impressions int, acos float64, budget string, ads SourceEvidence) Campaign {
	return Campaign{CampaignID: id, Name: name, SKU: skuID, ASIN: asin, Spend: spend, Sales: sales, Orders: orders, Clicks: clicks, Impressions: impressions, ACOS: acos, CPC: div(spend, float64(clicks)), CTR: div(float64(clicks), float64(impressions)), CVR: div(float64(orders), float64(clicks)), BudgetStatus: budget, Source: MetricSources{Ads: ads}}
}

func search(term, skuID, asin string, spend, sales float64, orders, clicks, impressions, rank int, share float64, action string, ads, brand SourceEvidence) SearchTerm {
	return SearchTerm{Term: term, SKU: skuID, ASIN: asin, Spend: spend, Sales: sales, Orders: orders, Clicks: clicks, Impressions: impressions, OrganicRank: rank, ClickShare: share, ConversionRate: div(float64(orders), float64(clicks)), AdAction: action, Source: MetricSources{Ads: ads, BrandAnalytics: brand}}
}

func listing(skuID, asin, title string, score float64, suppressed bool, defects []string, sessions int, cvr, adSpend float64, source SourceEvidence) ListingHealth {
	return ListingHealth{SKU: skuID, ASIN: asin, Title: title, Score: score, Suppressed: suppressed, Defects: defects, Sessions: sessions, ConversionRate: cvr, AdSpend: adSpend, Source: MetricSources{Listings: source}}
}

func margin(profit, revenue float64) float64 {
	if revenue == 0 {
		return 0
	}
	return profit / revenue
}

func div(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return a / b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
