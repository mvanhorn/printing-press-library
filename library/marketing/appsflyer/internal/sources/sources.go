// Package sources is the offline registry of AppsFlyer media-source identifiers.
//
// The catalog of canonical `_int` IDs and their display names is captured once
// from the AppsFlyer BETA MCP (list_cost_supported_media_sources and
// list_adrevenue_supported_media_sources) so the CLI can resolve user-typed
// names like "facebook" or "tiktok" to the canonical IDs the V2 API requires.
package sources

import (
	"sort"
	"strings"
)

// Source describes one media-source entry. Canonical is the API-accepted ID
// (e.g. "facebook_int"), Display is the human-readable name (e.g. "Meta ads"),
// and Aliases is a list of additional spellings users commonly type.
type Source struct {
	Canonical string
	Display   string
	Kind      string // "cost", "adrevenue", or "both"
	Aliases   []string
}

// Catalog returns the static registry of media sources. The list mirrors the
// AppsFlyer cost-supported and ad-revenue-supported integration catalogs.
func Catalog() []Source {
	return baseCatalog
}

// baseCatalog is the embedded source registry. Aliases were chosen from
// brand-name conventions; exhaustive enumeration would not survive in the
// codebase, so we cover the high-traffic sources and let `sources resolve`
// fall through to substring matching for the long tail.
var baseCatalog = []Source{
	{"facebook_int", "Meta ads", "cost", []string{"facebook", "fb", "meta"}},
	{"googleadwords_int", "Google Ads", "cost", []string{"google", "googleads", "adwords"}},
	{"tiktokglobal_int", "TikTok For Business", "cost", []string{"tiktok", "tt"}},
	{"iossearchads_int", "Apple Search Ads", "cost", []string{"apple", "asa", "applesearchads"}},
	{"snapchat_int", "Snapchat", "cost", []string{"snap"}},
	{"twitter_int", "X Ads (formerly Twitter Ads)", "cost", []string{"twitter", "x", "xads"}},
	{"reddit_int", "Reddit", "cost", []string{"reddit"}},
	{"pinterest_int", "Pinterest", "cost", []string{"pinterest"}},
	{"yahoogemini_int", "Yahoo", "cost", []string{"yahoo", "yahoogemini"}},
	{"applovin_int", "AppLovin", "cost", []string{"applovin", "lovin"}},
	{"applovinmax_int", "MAX", "adrevenue", []string{"max", "applovinmax", "applovin-max"}},
	{"vungle_int", "Vungle", "cost", []string{"vungle"}},
	{"unityads_int", "Unity Ads", "cost", []string{"unity", "unityads"}},
	{"unityadsmediation_int", "Unity Ads Mediation", "adrevenue", []string{"unitymediation"}},
	{"unitygv_int", "Unity CTV", "cost", []string{"unityctv"}},
	{"ironsource_int", "ironSource", "cost", []string{"ironsource", "is", "ironsrc"}},
	{"liftoff_int", "Liftoff", "cost", []string{"liftoff"}},
	{"jetfuelit_int", "Liftoff Influence", "cost", []string{"liftoffinfluence"}},
	{"chartboosts2s_int", "Chartboost", "cost", []string{"chartboost"}},
	{"mintegral_int", "Mintegral", "cost", []string{"mintegral"}},
	{"moloco_int", "Moloco", "cost", []string{"moloco"}},
	{"criteonew_int", "Criteo", "cost", []string{"criteo"}},
	{"appier_int", "Appier", "cost", []string{"appier"}},
	{"jampp_int", "Jampp", "cost", []string{"jampp"}},
	{"smadex_int", "Smadex", "cost", []string{"smadex"}},
	{"remerge_int", "Remerge", "cost", []string{"remerge"}},
	{"bidease_int", "Bidease", "cost", []string{"bidease"}},
	{"appgrowth_int", "Appgrowth", "cost", []string{"appgrowth"}},
	{"adgatemedia_int", "AdGate Media", "cost", []string{"adgate", "adgatemedia"}},
	{"adjoe_int", "adjoe GmbH", "cost", []string{"adjoe"}},
	{"buzzad_int", "Buzzvil", "cost", []string{"buzzvil", "buzzad"}},
	{"scrambly_int", "Scrambly", "cost", []string{"scrambly"}},
	{"tapjoy_int", "Tapjoy", "cost", []string{"tapjoy"}},
	{"mistplay_int", "Mistplay", "cost", []string{"mistplay"}},
	{"swagbucks_int", "Swagbucks Legacy", "cost", []string{"swagbucks"}},
	{"prodege_int", "Prodege Legacy", "cost", []string{"prodege"}},
	{"inboxdollars_int", "Prodege Main", "cost", []string{"inboxdollars"}},
	{"freecash_int", "Almedia GmbH", "cost", []string{"freecash", "almedia"}},
	{"joywallet_int", "Joywallet", "cost", []string{"joywallet"}},
	{"remoby_int", "Remoby", "cost", []string{"remoby"}},
	{"mail.ru_int", "VK Ads (ex. myTarget)", "cost", []string{"vk", "vkads", "mytarget"}},
	{"yandexdirect_int", "Yandex Direct", "cost", []string{"yandex", "yandexdirect"}},
	{"kakao_int", "Kakao", "cost", []string{"kakao"}},
	{"bytedance_int", "ByteDance Ads China - 巨量引擎", "cost", []string{"bytedance", "ocean"}},
	{"tencentams_int", "Tencent AMS", "cost", []string{"tencent", "tencentams"}},
	{"shareit_int", "SHAREit", "cost", []string{"shareit"}},
	{"digitalturbine_int", "Digital Turbine On Device", "cost", []string{"digitalturbine", "dt"}},
	{"onedigitalturbine_int", "Digital Turbine", "cost", []string{"onedigitalturbine"}},
	{"appia_int", "Digital Turbine Legacy", "cost", []string{"appia"}},
	{"xiaomiglobal_int", "Xiaomi Global", "cost", []string{"xiaomi"}},
	{"oppostore_int", "OPPO Store Traffic (Global)", "cost", []string{"oppo"}},
	{"samsungdsp_int", "Samsung DSP", "cost", []string{"samsung", "samsungdsp"}},
	{"avow_int", "Avow OEM", "cost", []string{"avow"}},
	{"inmobi_int", "InMobi", "cost", []string{"inmobi"}},
	{"inmobidsp_int", "InMobi DSP", "cost", []string{"inmobidsp"}},
	{"taptica_int", "Taptica", "cost", []string{"taptica"}},
	{"adtiming_int", "AdTiming", "cost", []string{"adtiming"}},
	{"appsamurai_int", "AppSamurai", "cost", []string{"appsamurai"}},
	{"hasoffers_int", "TUNE", "cost", []string{"tune", "hasoffers"}},
	{"motive_int", "Motive Interactive", "cost", []string{"motive"}},
	{"mobvista_int", "Nativex", "cost", []string{"nativex", "mobvista"}},
	{"appnext_int", "Appnext", "cost", []string{"appnext"}},
	{"appodeal_int", "Appodeal", "adrevenue", []string{"appodeal"}},
	{"admost_int", "Admost", "adrevenue", []string{"admost"}},
	{"tradplus_int", "TradePlus", "adrevenue", []string{"tradplus", "tradeplus"}},
	{"toponad_int", "Toponad", "adrevenue", []string{"toponad"}},
	{"sponsorpay_int", "Fyber", "both", []string{"fyber", "sponsorpay"}},
	{"odeeo_int", "Odeeo", "adrevenue", []string{"odeeo"}},
	{"googleadmob_int", "Google AdMob", "adrevenue", []string{"admob", "googleadmob"}},
	{"doubleclick_int", "Google Ad Manager (Ad Revenue)", "adrevenue", []string{"gam", "googleadmanager", "doubleclick"}},
	{"vidcoin_int", "Voodoo Ads", "adrevenue", []string{"voodoo", "vidcoin"}},
	{"bytedanceglobal_int", "Pangle (Ad Revenue)", "adrevenue", []string{"pangle"}},
	{"custom_mediation_int", "Custom Mediation", "adrevenue", []string{"custom", "custommediation"}},
	{"aura_int", "Aura from Unity", "cost", []string{"aura"}},
	{"adaction2_int", "AdAction Engage", "cost", []string{"adaction"}},
	{"adaction5_int", "Adaction3", "cost", []string{"adaction3"}},
	{"saygamesl0a_int", "SayGames Ltd", "cost", []string{"saygames"}},
}

// Resolve returns the canonical `_int` ID for the given user input. The
// resolver matches in order: exact canonical, exact display, exact alias,
// case-insensitive variants of each, then prefix match on display. Returns
// ("", false) when no candidate matches. Substring fallbacks deliberately
// require >=3 character input to avoid ambiguous one-letter triggers.
func Resolve(input string) (string, bool) {
	q := strings.ToLower(strings.TrimSpace(input))
	if q == "" {
		return "", false
	}
	for _, s := range baseCatalog {
		if strings.EqualFold(s.Canonical, q) {
			return s.Canonical, true
		}
	}
	for _, s := range baseCatalog {
		if strings.EqualFold(s.Display, q) {
			return s.Canonical, true
		}
		for _, a := range s.Aliases {
			if strings.EqualFold(a, q) {
				return s.Canonical, true
			}
		}
	}
	if len(q) >= 3 {
		for _, s := range baseCatalog {
			if strings.Contains(strings.ToLower(s.Display), q) {
				return s.Canonical, true
			}
		}
	}
	return "", false
}

// Search returns sources whose canonical ID, display name, or alias matches
// the query substring (case-insensitive). Empty query returns the full
// catalog sorted by display name.
func Search(query string) []Source {
	out := make([]Source, 0, len(baseCatalog))
	q := strings.ToLower(strings.TrimSpace(query))
	for _, s := range baseCatalog {
		if q == "" ||
			strings.Contains(strings.ToLower(s.Canonical), q) ||
			strings.Contains(strings.ToLower(s.Display), q) {
			out = append(out, s)
			continue
		}
		for _, a := range s.Aliases {
			if strings.Contains(strings.ToLower(a), q) {
				out = append(out, s)
				break
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Display < out[j].Display })
	return out
}
