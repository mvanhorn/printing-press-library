package jalan

import (
	"regexp"
	"strings"
)

var (
	outdoorRoomRE     = regexp.MustCompile(`露天(?:風呂)?付|露天風呂付き|(?:客室|お部屋|全室)[^。\n]*露天風呂|露天風呂[^。\n]*客室`)
	noOutdoorRE       = regexp.MustCompile(`露天(?:風呂)?(?:は)?(?:なし|無し|ありません|ございません)`)
	noIndoorRE        = regexp.MustCompile(`バス(?:は)?(?:なし|無し|無)|内(?:湯|風呂)(?:は)?(?:なし|無し|ありません|ございません)`)
	noRoomBathroomRE  = regexp.MustCompile(`(?:室内|客室内|お部屋)(?:には|に|の)[[:space:]、]*(?:浴室|内風呂)(?:は|が)?(?:ございません|ありません|ない|無い|なし|無し)|(?:浴室|内風呂)(?:は|が)?(?:室内|客室内|お部屋)(?:には|に|の)[[:space:]、]*(?:ございません|ありません|ない|無い|なし|無し)`)
	outsideRoomRE     = regexp.MustCompile(`(?:室内|客室内|お部屋)(?:以外|の外)`)
	indoorRE          = regexp.MustCompile(`バス付|バストイレ|内(?:湯|風呂)付|(?:室内|客室内|お部屋)[^。\n]*(?:浴室|内風呂)`)
	noRoomHotSpringRE = regexp.MustCompile(`温泉(?:で|では)(?:は)?(?:ございません|ありません|ない)|温泉(?:は)?(?:なし|無し)|沸かし湯`)
	roomHotSpringRE   = regexp.MustCompile(`温泉[^。\n]{0,20}(?:露天|風呂|バス|浴室)|(?:露天|風呂|バス|浴室)[^。\n]{0,20}温泉|温泉かけ流し|温泉掛け流し`)
)

// roomBaths examines only room names, room labels and room descriptions. It
// never receives property descriptions or a plan's shared-bath advertisement.
func roomBaths(parts []string, sourceURL string) BathFacts {
	b := emptyBaths()
	indoorNegated := false
	indoorPositiveEvidence := []string{}
	for _, part := range parts {
		for _, s := range regexp.MustCompile(`[。\n]`).Split(part, -1) {
			s = clean(s)
			if s == "" {
				continue
			}
			if outdoorRoomRE.MatchString(s) && !noOutdoorRE.MatchString(s) {
				b.Outdoor = boolPtr(true)
				addEvidence(&b.Evidence, "baths.outdoor", s, sourceURL)
			} else if noOutdoorRE.MatchString(s) {
				b.Outdoor = boolPtr(false)
				addEvidence(&b.Evidence, "baths.outdoor", s, sourceURL)
			}
			if noIndoorRE.MatchString(s) || noRoomBathroomRE.MatchString(s) {
				indoorNegated = true
				b.Indoor = boolPtr(false)
				addEvidence(&b.Evidence, "baths.indoor", s, sourceURL)
			} else if !indoorNegated && !outsideRoomRE.MatchString(s) && indoorRE.MatchString(s) {
				b.Indoor = boolPtr(true)
				indoorPositiveEvidence = append(indoorPositiveEvidence, s)
			}
			if noRoomHotSpringRE.MatchString(s) {
				b.HotSpring = boolPtr(false)
				addEvidence(&b.Evidence, "baths.hot_spring", s, sourceURL)
			} else if roomHotSpringRE.MatchString(s) && (b.HotSpring == nil || *b.HotSpring) {
				b.HotSpring = boolPtr(true)
				addEvidence(&b.Evidence, "baths.hot_spring", s, sourceURL)
			}
		}
	}
	if b.Indoor != nil && *b.Indoor {
		for _, s := range indoorPositiveEvidence {
			addEvidence(&b.Evidence, "baths.in_room", s, sourceURL)
		}
	}
	if (b.Indoor != nil && *b.Indoor) || (b.Outdoor != nil && *b.Outdoor) {
		b.InRoom = boolPtr(true)
	}
	return b
}

func propertyBaths(rows map[string]string, sourceURL string) BathFacts {
	b := emptyBaths()
	if s := rows["温泉"]; s != "" {
		if strings.HasPrefix(s, "なし") || strings.HasPrefix(s, "無し") {
			b.HotSpring = boolPtr(false)
		} else {
			b.HotSpring = boolPtr(true)
		}
		addEvidence(&b.Evidence, "baths.hot_spring", s, sourceURL)
	}
	if s := rows["露天風呂"]; s != "" {
		if strings.HasPrefix(s, "あり") {
			b.Outdoor = boolPtr(true)
		} else if strings.HasPrefix(s, "なし") || strings.HasPrefix(s, "無し") {
			b.Outdoor = boolPtr(false)
		}
		addEvidence(&b.Evidence, "baths.outdoor", s, sourceURL)
	}
	if s := rows["貸切風呂"]; s != "" {
		if strings.HasPrefix(s, "あり") {
			b.Private = boolPtr(true)
		} else if strings.HasPrefix(s, "なし") || strings.HasPrefix(s, "無し") {
			b.Private = boolPtr(false)
			b.PrivateReservable = boolPtr(false)
		}
		addEvidence(&b.Evidence, "baths.private", s, sourceURL)
	}
	if s := rows["風呂利用条件"]; s != "" {
		if strings.Contains(s, "貸切") {
			if strings.Contains(s, "ご予約制ではございません") || strings.Contains(s, "予約不要") || strings.Contains(s, "予約制ではありません") || strings.Contains(s, "予約不可") {
				b.PrivateReservable = boolPtr(false)
			} else if strings.Contains(s, "予約制") || strings.Contains(s, "要予約") || strings.Contains(s, "予約が必要") {
				b.PrivateReservable = boolPtr(true)
			}
		}
		addEvidence(&b.Evidence, "baths.conditions", s, sourceURL)
	}
	return b
}

// smoking receives current-room labels/names before descriptive prose. A
// negated declaration never becomes a positive non-smoking fact.
func smoking(parts []string) string {
	nonSmoking, smokingRoom := false, false
	for _, s := range parts {
		s = clean(s)
		negative := strings.Contains(s, "禁煙ではありません") || strings.Contains(s, "禁煙ではございません") || strings.Contains(s, "禁煙ではない") || strings.Contains(s, "禁煙不可")
		if !negative && (s == "禁煙" || strings.Contains(s, "禁煙ルーム") || strings.Contains(s, "禁煙室") || strings.Contains(s, "全室禁煙") || strings.Contains(s, "【禁煙】") || strings.Contains(s, "[禁煙]") || strings.Contains(s, "喫煙不可")) {
			nonSmoking = true
		}
		if !strings.Contains(s, "喫煙不可") && (s == "喫煙" || strings.Contains(s, "喫煙ルーム") || strings.Contains(s, "喫煙室") || strings.Contains(s, "【喫煙】") || strings.Contains(s, "[喫煙]")) {
			smokingRoom = true
		}
	}
	if nonSmoking == smokingRoom {
		return "unknown"
	}
	if nonSmoking {
		return "non_smoking"
	}
	return "smoking"
}

func meals(s string) string {
	if strings.Contains(s, "朝・夕") || strings.Contains(s, "朝夕") || strings.Contains(s, "２食") || strings.Contains(s, "2食") {
		return "breakfast_dinner"
	}
	if strings.Contains(s, "食事なし") || strings.Contains(s, "食事無し") || strings.Contains(s, "素泊まり") {
		return "none"
	}
	if strings.Contains(s, "朝") && !strings.Contains(s, "夕") {
		return "breakfast"
	}
	if strings.Contains(s, "夕") && !strings.Contains(s, "朝") {
		return "dinner"
	}
	return "unknown"
}
