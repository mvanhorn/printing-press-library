package serpparse

import (
	"strings"
	"testing"
)

// sampleSERPLegacy uses the old api_txt_lines markup. Kept as a
// regression fixture so the legacy fallback path stays exercised.
const sampleSERPLegacy = `<html><body>
<ul class="lst_total">
  <li class="bx">
    <a href="https://m.blog.naver.com/selly9401/224234460263" class="api_txt_lines total_tit">
      <span class="api_txt_lines total_tit">칠리 협찬 후기 — 솔직 리뷰</span>
    </a>
    <span class="api_txt_lines dsc_txt">진짜 솔직하게 써본 칠리 신상품 후기입니다.</span>
  </li>
  <li class="bx">
    <a href="https://m.blog.naver.com/foodie_kim/223999888777">
      <span class="api_txt_lines total_tit">다른 칠리 글</span>
    </a>
    <span class="api_txt_lines dsc_txt">또 다른 후기...</span>
  </li>
  <!-- Dup of first hit (more-from-this-blog sub-block) -->
  <a href="https://m.blog.naver.com/selly9401/224234460263">duplicate</a>
</ul>
</body></html>`

// sampleSERPModern reproduces the 2026-05 mobile SERP markup:
// each blog hit is wrapped in `<div data-template-id="ugcItem">`
// with sds-comps-text-type-headline1 for the title and
// sds-comps-text-type-body1 for the snippet. Query highlights are
// wrapped in `<mark>` which the parser must strip.
const sampleSERPModern = `<html><body>
<div class="api_subject_bx">
  <div data-template-id="ugcItem">
    <a href="https://m.blog.naver.com/selly9401/224234460263">
      <span class="sds-comps-text sds-comps-text-ellipsis-2 sds-comps-text-type-headline1 sds-comps-text-weight-sm">이탈리아 <mark>여성청결제</mark> <mark>칠리</mark> 사용 후기</span>
    </a>
    <a href="https://m.blog.naver.com/selly9401/224234460263">
      <span class="sds-comps-text sds-comps-text-type-body1 sds-comps-text-weight-sm">진짜 솔직하게 써본 <mark>칠리</mark> 신상품 후기입니다. 약산성 케어가 좋아요.</span>
    </a>
  </div>
  <div data-template-id="ugcItem">
    <a href="https://m.blog.naver.com/foodie_kim/223999888777">
      <span class="sds-comps-text sds-comps-text-type-headline1">다른 <mark>칠리</mark> 글</span>
    </a>
    <a href="https://m.blog.naver.com/foodie_kim/223999888777">
      <span class="sds-comps-text sds-comps-text-type-body1">또 다른 <mark>칠리</mark> 후기 본문 발췌...</span>
    </a>
  </div>
  <!-- "more from this blog" rail outside any ugcItem; should be deduped -->
  <a href="https://m.blog.naver.com/selly9401/224234460263">duplicate</a>
</div>
</body></html>`

func TestParseSERPModern(t *testing.T) {
	got, err := ParseSERP([]byte(sampleSERPModern), "칠리")
	if err != nil {
		t.Fatalf("ParseSERP: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(results) = %d, want 2 (deduped)", len(got))
	}

	r0 := got[0]
	if r0.Rank != 1 {
		t.Errorf("results[0].Rank = %d, want 1", r0.Rank)
	}
	if r0.URL != "https://m.blog.naver.com/selly9401/224234460263" {
		t.Errorf("results[0].URL = %q", r0.URL)
	}
	if r0.BlogID != "selly9401" || r0.LogNo != "224234460263" {
		t.Errorf("results[0] canonical key = (%q, %q)", r0.BlogID, r0.LogNo)
	}
	if !strings.Contains(r0.Title, "이탈리아 여성청결제 칠리") {
		t.Errorf("results[0].Title = %q (mark stripping failed?)", r0.Title)
	}
	if strings.Contains(r0.Title, "<mark>") || strings.Contains(r0.Title, "</mark>") {
		t.Errorf("results[0].Title still contains <mark> tags: %q", r0.Title)
	}
	if !strings.Contains(r0.Snippet, "솔직하게 써본 칠리") {
		t.Errorf("results[0].Snippet = %q", r0.Snippet)
	}

	r1 := got[1]
	if r1.Rank != 2 {
		t.Errorf("results[1].Rank = %d, want 2", r1.Rank)
	}
	if r1.BlogID != "foodie_kim" || r1.LogNo != "223999888777" {
		t.Errorf("results[1] canonical key = (%q, %q)", r1.BlogID, r1.LogNo)
	}
	if !strings.Contains(r1.Title, "다른 칠리 글") {
		t.Errorf("results[1].Title = %q", r1.Title)
	}
}

func TestParseSERPLegacyFallback(t *testing.T) {
	got, err := ParseSERP([]byte(sampleSERPLegacy), "칠리")
	if err != nil {
		t.Fatalf("ParseSERP: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(results) = %d, want 2 (deduped)", len(got))
	}
	if !strings.Contains(got[0].Title, "칠리 협찬 후기") {
		t.Errorf("legacy fallback title = %q", got[0].Title)
	}
	if !strings.Contains(got[0].Snippet, "솔직하게") {
		t.Errorf("legacy fallback snippet = %q", got[0].Snippet)
	}
}

func TestParseSERPEmpty(t *testing.T) {
	if _, err := ParseSERP(nil, ""); err == nil {
		t.Error("expected error for nil input")
	}
}

func TestParseSERPNoHits(t *testing.T) {
	src := `<html><body><div>no blog URLs here</div></body></html>`
	got, err := ParseSERP([]byte(src), "abc")
	if err != nil {
		t.Fatalf("ParseSERP: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len(results) = %d, want 0", len(got))
	}
}

func TestParseSERPMissingTitleAnchorFallback(t *testing.T) {
	src := `<html><body>
<a href="https://m.blog.naver.com/anchorblog/224999000111">제목만 앵커</a>
</body></html>`
	got, err := ParseSERP([]byte(src), "x")
	if err != nil {
		t.Fatalf("ParseSERP: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(results) = %d", len(got))
	}
	if !strings.Contains(got[0].Title, "제목만 앵커") {
		t.Errorf("title fallback failed: %q", got[0].Title)
	}
}
