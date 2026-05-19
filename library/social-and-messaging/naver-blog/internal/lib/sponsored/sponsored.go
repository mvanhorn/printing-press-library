// Package sponsored detects whether a Naver Blog post body contains
// KFTC-required sponsored-content disclosure language.
//
// Korea's Fair Trade Commission (공정거래위원회, KFTC) requires
// influencers receiving free product, payment, or service from a
// brand to disclose that relationship in the body of any post about
// the brand. The required phrasing isn't standardized — every brand
// and influencer agency uses slight variants — but a stable finite
// set of marker phrases covers the overwhelming majority of
// disclosures in the wild.
//
// This package scans for that finite set. The detector is
// intentionally narrow (precision over recall): missing a disclosure
// is preferable to mis-flagging a legitimate organic post as
// sponsored. The "본 포스팅은 ... 받아 ... 작성" regex catches the
// long-form variant where the literal markers are spelled out across
// a longer sentence.
package sponsored

import (
	"regexp"
	"strings"
)

// Literal markers — substring match, no regex. Order is preserved in
// MarkersMatched so callers see them in the order they appear in this
// slice (NOT in document order). Document order would require a more
// expensive scan with no downstream benefit.
var literalMarkers = []string{
	"협찬",       // sponsorship
	"체험단",      // experience group / trial program
	"광고 포함",    // advertisement included
	"유료광고 포함",  // paid advertisement included (no space)
	"유료 광고 포함", // paid advertisement included (with space)
	"유료광고입니다",  // KFTC alternate: "is a paid advertisement"
	"유료 광고입니다", // (with space)
	"원고료",      // payment for the post (extremely common KFTC marker)
	"소정의 원고료",  // formal payment-acknowledgement phrase
}

// reSentenceForm matches the long-form disclosure that brands ask
// influencers to drop into the opening or closing paragraph. Examples
// it catches:
//
//	"본 포스팅은 [브랜드]로부터 제품을 제공받아 작성된 글입니다."
//	"본 포스팅은 무상으로 제공받아 작성하였습니다."
//	"본 콘텐츠는 [브랜드]사로부터 제품을 제공받아 작성된 유료광고입니다."
//	"본 리뷰는 ... 받아 작성"
//
// The `[^.]{0,200}?` middle bound prevents the regex from matching
// across multiple sentences (so "본 포스팅은 칠리에 대한 글." plus a
// later "받아 작성" wouldn't false-positive). 200 chars covers the
// real-world brand-name-plus-product-line interpolation that pushes
// the disclosure past the original 100-char window.
//
// The opener token has been generalized from "본 포스팅" to also
// accept "본 콘텐츠" / "본 리뷰" / "본 글" / "본 게시물" / "본 포스트"
// since the literal opener varies by brand template.
var reSentenceForm = regexp.MustCompile(`본\s*(포스팅|콘텐츠|리뷰|글|게시물|포스트)[은는]\s+[^.]{0,200}?(받아|제공받아|지원받아|받고)\s*작성`)

// DetectSponsored returns true when bodyText carries one or more
// disclosure markers. markersMatched lists which markers fired, in
// the order they appear in literalMarkers, followed by the verbatim
// substring matched by the long-form sentence regex if any. The list
// is deduplicated.
//
// All entries are verbatim substrings from the post body — never
// synthetic pattern labels. Callers can rely on each marker being
// directly searchable in the original text.
//
// Pass the plain-text body extracted by postparse.stripHTMLToText, not
// raw HTML — substring scans against HTML produce false positives
// when class names or alt text mention "협찬" without the post body
// actually disclosing sponsorship.
func DetectSponsored(bodyText string) (bool, []string) {
	if bodyText == "" {
		return false, nil
	}
	var matched []string
	seen := make(map[string]bool)
	for _, marker := range literalMarkers {
		if strings.Contains(bodyText, marker) && !seen[marker] {
			seen[marker] = true
			matched = append(matched, marker)
		}
	}
	if loc := reSentenceForm.FindStringIndex(bodyText); loc != nil {
		// Emit the verbatim matched substring so callers can search
		// for it in the original body. Avoids leaking a synthetic
		// pattern label (e.g. "본 포스팅은 ... 작성") that doesn't
		// exist in the source.
		actualMatch := bodyText[loc[0]:loc[1]]
		if !seen[actualMatch] {
			seen[actualMatch] = true
			matched = append(matched, actualMatch)
		}
	}
	return len(matched) > 0, matched
}
