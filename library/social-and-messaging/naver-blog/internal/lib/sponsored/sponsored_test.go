package sponsored

import (
	"strings"
	"testing"
)

func TestDetectSponsoredPositiveLiterals(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"협찬 alone", "오늘은 협찬 받아서 써본 제품입니다.", "협찬"},
		{"체험단", "체험단으로 참여한 글입니다.", "체험단"},
		{"광고 포함", "이 글은 광고 포함 콘텐츠입니다.", "광고 포함"},
		{"유료광고 포함 no space", "유료광고 포함된 후기.", "유료광고 포함"},
		{"유료 광고 포함 with space", "유료 광고 포함된 콘텐츠.", "유료 광고 포함"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ok, markers := DetectSponsored(tc.body)
			if !ok {
				t.Fatalf("DetectSponsored = false, want true")
			}
			found := false
			for _, m := range markers {
				if m == tc.want {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("markers = %v, expected to include %q", markers, tc.want)
			}
		})
	}
}

func TestDetectSponsoredSentenceForm(t *testing.T) {
	body := "안녕하세요. 본 포스팅은 칠리로부터 제품을 제공받아 작성된 글입니다. 즐거운 하루 되세요."
	ok, markers := DetectSponsored(body)
	if !ok {
		t.Fatalf("DetectSponsored = false, want true")
	}
	if len(markers) == 0 || !strings.Contains(markers[len(markers)-1], "본 포스팅은") {
		t.Errorf("markers = %v, expected sentence-form match", markers)
	}
}

func TestDetectSponsoredSentenceFormVariants(t *testing.T) {
	bodies := []string{
		"본 포스팅은 무상으로 제공받아 작성하였습니다.",
		"본 포스팅은 브랜드 지원받아 작성된 글.",
		"본 포스팅은 협찬을 받아 작성된 후기입니다.",
	}
	for _, b := range bodies {
		t.Run(b, func(t *testing.T) {
			ok, _ := DetectSponsored(b)
			if !ok {
				t.Errorf("DetectSponsored = false for %q", b)
			}
		})
	}
}

func TestDetectSponsoredNegative(t *testing.T) {
	cases := []string{
		"오늘 직접 구매한 칠리 신상품을 써봤습니다.",
		"이 글은 그냥 일기입니다. 광고가 아닙니다.",
		"",
	}
	for _, body := range cases {
		t.Run(body, func(t *testing.T) {
			ok, markers := DetectSponsored(body)
			if ok {
				t.Errorf("DetectSponsored = true for %q (markers=%v), want false", body, markers)
			}
		})
	}
}

func TestDetectSponsoredSentenceFormCrossSentenceNoMatch(t *testing.T) {
	// The "본 포스팅은" sentence ends with a period, so a later
	// "받아 ... 작성" must NOT cross the sentence boundary.
	body := "본 포스팅은 칠리에 대한 일기입니다. 작년에 친구한테 받아 직접 작성한 글이에요."
	ok, _ := DetectSponsored(body)
	if ok {
		t.Errorf("DetectSponsored matched across a sentence break: %q", body)
	}
}
