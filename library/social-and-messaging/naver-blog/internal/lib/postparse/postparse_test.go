package postparse

import (
	"strings"
	"testing"
	"time"
)

const sampleMobileHTML = `<!doctype html>
<html lang="ko">
<head>
<meta property="og:title" content="협찬 받은 칠리 신상 후기 — 진짜 솔직 리뷰">
<meta property="og:description" content="이번에 칠리에서 새로 출시한 신상품을 협찬으로 받아서 직접 사용해본 후기를 남겨봅니다.">
<meta property="og:image" content="https://blogimgs.pstatic.net/foo/bar.jpg">
<script>
var gsTagName = '협찬,칠리,신상,후기';
</script>
</head>
<body>
<div class="se-main-container">
  <p>안녕하세요 여러분! 오늘은 <strong>칠리</strong> 신상품을 소개해드릴게요.</p>
  <div class="se-section">
    <p>이 제품은 <em>협찬</em>을 받아 작성된 글입니다.</p>
  </div>
</div>
<div class="footer">unrelated content</div>
</body>
</html>`

func TestParseMobilePost(t *testing.T) {
	meta, err := ParseMobilePost([]byte(sampleMobileHTML))
	if err != nil {
		t.Fatalf("ParseMobilePost: %v", err)
	}
	if !strings.Contains(meta.Title, "협찬 받은 칠리") {
		t.Errorf("title = %q, want a Chilly sponsorship title", meta.Title)
	}
	if !strings.Contains(meta.Description, "신상품") {
		t.Errorf("description = %q, want a snippet mentioning the new product", meta.Description)
	}
	if meta.ThumbnailURL != "https://blogimgs.pstatic.net/foo/bar.jpg" {
		t.Errorf("thumbnail_url = %q", meta.ThumbnailURL)
	}
	wantTags := []string{"협찬", "칠리", "신상", "후기"}
	if len(meta.Tags) != len(wantTags) {
		t.Fatalf("len(tags) = %d, want %d (%v)", len(meta.Tags), len(wantTags), meta.Tags)
	}
	for i, tag := range wantTags {
		if meta.Tags[i] != tag {
			t.Errorf("tags[%d] = %q, want %q", i, meta.Tags[i], tag)
		}
	}
	if !strings.Contains(meta.BodyHTML, "칠리") || !strings.Contains(meta.BodyHTML, "se-section") {
		t.Errorf("body_html doesn't include expected inner content: %q", meta.BodyHTML)
	}
	if strings.Contains(meta.BodyHTML, "footer") {
		t.Errorf("body_html bled into footer div: %q", meta.BodyHTML)
	}
	if !strings.Contains(meta.BodyText, "칠리") || !strings.Contains(meta.BodyText, "협찬") {
		t.Errorf("body_text missing core content: %q", meta.BodyText)
	}
	if strings.Contains(meta.BodyText, "<strong>") {
		t.Errorf("body_text still contains HTML tags: %q", meta.BodyText)
	}
}

func TestParseMobilePostEmpty(t *testing.T) {
	if _, err := ParseMobilePost(nil); err == nil {
		t.Error("expected error for nil input")
	}
	if _, err := ParseMobilePost([]byte("")); err == nil {
		t.Error("expected error for empty input")
	}
}

func TestParseMobilePostNoBody(t *testing.T) {
	src := `<meta property="og:title" content="Title"><body>no main container here</body>`
	meta, err := ParseMobilePost([]byte(src))
	if err != nil {
		t.Fatalf("ParseMobilePost: %v", err)
	}
	if meta.Title != "Title" {
		t.Errorf("title = %q", meta.Title)
	}
	if meta.BodyHTML != "" {
		t.Errorf("body_html should be empty when no se-main-container, got %q", meta.BodyHTML)
	}
}

func TestParseMobilePostExtractsContentImages(t *testing.T) {
	src := `<html><body>
<div class="se-main-container">
  <img src="https://postfiles.pstatic.net/MjAy/image-one.jpg?type=w966">
  <img src="https://postfiles.pstatic.net/MjAy/image-one.jpg?type=w966">
  <img src="https://blogfiles.pstatic.net/MjAy/image-two.png">
  <img src="https://blogthumb.pstatic.net/MjAy/image-three.jpg">
  <img src="https://blogimgs.pstatic.net/imgs/emot/emo12.gif">
  <img src="https://postfiles.pstatic.net/sticker/sticker.png">
  <img src="https://postfiles.pstatic.net/profile/profile.jpg">
</div>
</body></html>`
	meta, err := ParseMobilePost([]byte(src))
	if err != nil {
		t.Fatalf("ParseMobilePost: %v", err)
	}
	want := []string{
		"https://postfiles.pstatic.net/MjAy/image-one.jpg?type=w966",
		"https://blogfiles.pstatic.net/MjAy/image-two.png",
		"https://blogthumb.pstatic.net/MjAy/image-three.jpg",
	}
	if len(meta.Images) != len(want) {
		t.Fatalf("len(images) = %d, want %d (%v)", len(meta.Images), len(want), meta.Images)
	}
	for i := range want {
		if meta.Images[i] != want[i] {
			t.Errorf("images[%d] = %q, want %q", i, meta.Images[i], want[i])
		}
	}
}

const samplePostViewHTML = `<html>
<body>
<span class="se_publishDate pcol2"> 2026. 3. 30. 15:35</span>
<em id="commentCount" class="num_cmt">12</em>
<em id="floating_bottom_commentCount" class="num_cmt">12</em>
</body>
</html>`

func TestParsePostView(t *testing.T) {
	meta, err := ParsePostView([]byte(samplePostViewHTML))
	if err != nil {
		t.Fatalf("ParsePostView: %v", err)
	}
	if meta.CommentCount != 12 {
		t.Errorf("comment_count = %d, want 12", meta.CommentCount)
	}
	if meta.FloatingCommentCount != 12 {
		t.Errorf("floating_comment_count = %d, want 12", meta.FloatingCommentCount)
	}
	if meta.PublishDateStr != "2026. 3. 30. 15:35" {
		t.Errorf("publish_date_str = %q", meta.PublishDateStr)
	}
	if meta.PublishedAtUTC.IsZero() {
		t.Fatal("published_at_utc is zero — should be parsed as KST 2026-03-30 15:35")
	}
	// KST 15:35 = UTC 06:35.
	wantHour, wantMin := 6, 35
	if meta.PublishedAtUTC.Hour() != wantHour || meta.PublishedAtUTC.Minute() != wantMin {
		t.Errorf("published_at_utc time = %02d:%02d, want %02d:%02d",
			meta.PublishedAtUTC.Hour(), meta.PublishedAtUTC.Minute(), wantHour, wantMin)
	}
	if meta.PublishedAtUTC.Location() != time.UTC {
		t.Errorf("location = %v, want UTC", meta.PublishedAtUTC.Location())
	}
}

func TestParsePostViewEmptyEm(t *testing.T) {
	// Naver renders posts with zero comments as an empty <em>.
	src := `<em id="commentCount" class="num_cmt"></em>`
	meta, err := ParsePostView([]byte(src))
	if err != nil {
		t.Fatalf("ParsePostView: %v", err)
	}
	if meta.CommentCount != 0 {
		t.Errorf("comment_count = %d, want 0 (empty em)", meta.CommentCount)
	}
}

func TestParsePostViewMissingEm(t *testing.T) {
	src := `<html><body>no em tag at all</body></html>`
	meta, err := ParsePostView([]byte(src))
	if err != nil {
		t.Fatalf("ParsePostView: %v", err)
	}
	if meta.CommentCount != 0 {
		t.Errorf("comment_count = %d, want 0 (no em)", meta.CommentCount)
	}
	if !meta.PublishedAtUTC.IsZero() {
		t.Errorf("published_at_utc should be zero when no date is found")
	}
}
