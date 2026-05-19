package cli

import (
	"encoding/json"
	"testing"
)

func TestProjectFeedItemSurfacesGeneralPurposeFields(t *testing.T) {
	readCount := 77
	got := projectFeedItem("blog", rawFeedItem{
		LogNo:                   json.Number("224234460263"),
		TitleWithInspectMessage: "title",
		BriefContents:           "brief",
		MemoLog:                 true,
		PlaceName:               "서울 맛집",
		MarketPost:              true,
		VideoPlayTime:           93,
		IsVRThumbnail:           true,
		IsVideoThumbnail:        true,
		CategoryNo:              11,
		CategoryName:            "맛집",
		SympathyCount:           3,
		CommentCount:            4,
		ShareCount:              5,
		ReadCount:               &readCount,
		OpenGraphLink:           "https://m.blog.naver.com/blog/224234460263",
		ScrapType:               2,
		AddDate:                 json.Number("1716090000000"),
		ThumbnailURL:            "https://thumb.example/t.jpg",
	})
	if got.ReadCount == nil || *got.ReadCount != 77 {
		t.Fatalf("read_count = %v, want pointer to 77", got.ReadCount)
	}
	if !got.MemoLog || got.PlaceName != "서울 맛집" || !got.MarketPost {
		t.Errorf("memo/place/market fields = %+v", got)
	}
	if got.VideoPlayTime != 93 || !got.IsVRThumbnail || !got.IsVideoThumbnail {
		t.Errorf("video thumbnail fields = %+v", got)
	}
	if got.CategoryNo != 11 || got.OpenGraphLink == "" || got.ScrapType != 2 {
		t.Errorf("category/share/privacy fields = %+v", got)
	}
}

func TestProjectFeedItemKeepsNullReadCount(t *testing.T) {
	got := projectFeedItem("blog", rawFeedItem{LogNo: json.Number("1")})
	if got.ReadCount != nil {
		t.Fatalf("read_count = %v, want nil", got.ReadCount)
	}
}

func TestNormalizeBlogSort(t *testing.T) {
	for _, raw := range []string{"", "recent", "best"} {
		if _, err := normalizeBlogSort(raw); err != nil {
			t.Fatalf("normalizeBlogSort(%q): %v", raw, err)
		}
	}
	if _, err := normalizeBlogSort("popular"); err == nil {
		t.Fatal("expected error for invalid sort")
	}
}
