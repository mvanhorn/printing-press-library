// Copyright 2026 and contributors. Licensed under Apache-2.0. See LICENSE.
package awwwards

import "testing"

const cardFixture = `<ul class="grid-cards js-ajax-entries"><li class="col-3 js-collectable" data-controller="collectable" data-collectable-model-value="{&quot;collectableIdentifier&quot;:&quot;monolog&quot;,&quot;collectableImage&quot;:&quot;submissions/2026/05/6a17a0bfd8cba363259166.jpg&quot;,&quot;collectableTitle&quot;:&quot;MONOLOG&quot;,&quot;id&quot;:64965,&quot;images&quot;:{&quot;thumbnail&quot;:&quot;submissions/2026/05/6a17a0bfd8cba363259166.jpg&quot;},&quot;slug&quot;:&quot;monolog&quot;,&quot;title&quot;:&quot;MONOLOG&quot;,&quot;createdAt&quot;:1783296000,&quot;tags&quot;:[&quot;Design Agencies&quot;,&quot;Clean&quot;,&quot;GSAP&quot;,&quot;Three.js&quot;,&quot;Webflow&quot;],&quot;type&quot;:&quot;submission&quot;}"><div class="card-site"><a href="/sites/monolog">MONOLOG</a></div></li></ul>`

const elementCardFixture = `<li class="col-3 js-collectable" data-collectable-model-value="{&quot;collectableIdentifier&quot;:&quot;ac35f119-8227-4588-acca-9cceafed6c9a&quot;,&quot;collectableImage&quot;:&quot;element/2026/05/6a0ca0b1ba7e3045691902.png&quot;,&quot;collectableTitle&quot;:&quot;Hero&quot;,&quot;id&quot;:&quot;ac35f119-8227-4588-acca-9cceafed6c9a&quot;,&quot;user&quot;:{&quot;image&quot;:&quot;avatar/2234659/x.png&quot;,&quot;username&quot;:&quot;mdxinc&quot;,&quot;displayName&quot;:&quot;MDX&quot;,&quot;type&quot;:&quot;user&quot;},&quot;main_image&quot;:&quot;element/2026/05/6a0ca0b1ba7e3045691902.png&quot;,&quot;tags&quot;:[&quot;hero&quot;,&quot;luxury hero&quot;],&quot;title&quot;:&quot;Hero&quot;,&quot;createdAt&quot;:1779212465,&quot;type&quot;:&quot;element&quot;}"><a href="/inspiration/hero-tower-garage-doors">Hero</a> <a href="http://towerdoors.com.au">visit</a></li>`

const detailFixture = `<html><head><title>&#128052; MONOLOG - Awwwards SOTD</title></head><body>
<h1 class="heading-1 text-uppercase"> <a href="https://bymonolog.com/" target="_blank" rel="noopener">MONOLOG</a> </h1>
<div class="users-credits"><ul class="users-credits__details"><li><a class="avatar-name__link" href="/byhuy/" aria-label="Huy Nguyen"><strong>Huy Nguyen</strong></a></li><li><a class="avatar-name__link" href="/monolog/" aria-label="MONOLOG"><strong>MONOLOG</strong></a></li></ul></div>
<div class="layout-overall__progressbar js-chart-bar" data-note="7.43"></div>
<div class="layout-overall__score"><strong>7.61 / 10</strong></div>
<div class="layout-overall__score"><strong>7.29 / 10</strong></div>
<div class="layout-overall__score"><strong>7.2 / 10</strong></div>
<div class="layout-overall__score"><strong>7.54 / 10</strong></div>
<ul class="list-jury-notes"><li class="js-hidden-list-element list-jury-notes__item "><div class="list-jury-notes__info"><div class="info"><div><a href="/lisovskiy/"><strong>Siarhiej Lisouski</strong></a><span class="list-jury-notes__from"> from <strong>Georgia</strong></span></div></div></div><div class="grid-score"><div class="grid-score__item">8</div><div class="grid-score__item">7.5</div><div class="grid-score__item">7</div><div class="grid-score__item">8</div><div class="grid-score__item grid-score__item--total">7.75</div></div></li></ul>
<ul class="list-palette"><li><div class="list-palette__header"><div class="list-palette__name"><strong>HEX</strong> #080807</div></div></li><li><div class="list-palette__name"><strong>HEX</strong> #DDDDD5</div></li></ul>
<h2 class="heading-section__title">Technologies &amp; Tools</h2><ul class="list-tags"><li><strong><a href="/websites/design-agencies/" class="button button--tag">Design Agencies</a></strong></li><li><strong><a href="/websites/gsap/" class="button button--tag">GSAP</a></strong></li></ul>
</body></html>`

func TestParseCards(t *testing.T) {
	cards := ParseCards(cardFixture)
	if len(cards) != 1 {
		t.Fatalf("want 1 card, got %d", len(cards))
	}
	c := cards[0]
	if c.Slug != "monolog" || c.Title != "MONOLOG" {
		t.Errorf("slug/title = %q/%q", c.Slug, c.Title)
	}
	if c.CreatedAt != 1783296000 {
		t.Errorf("createdAt = %d", c.CreatedAt)
	}
	if len(c.Tags) != 5 || c.Tags[2] != "GSAP" {
		t.Errorf("tags = %v", c.Tags)
	}
	if got := ThumbnailURL(c.Thumbnail(), ""); got != "https://assets.awwwards.com/awards/media/cache/thumb_440_330/submissions/2026/05/6a17a0bfd8cba363259166.jpg" {
		t.Errorf("thumbnail url = %q", got)
	}
}

func TestParseCardsElement(t *testing.T) {
	cards := ParseCards(elementCardFixture)
	if len(cards) != 1 {
		t.Fatalf("want 1 element card, got %d", len(cards))
	}
	c := cards[0]
	if c.Type != "element" || c.Title != "Hero" {
		t.Errorf("type/title = %q/%q", c.Type, c.Title)
	}
	if c.User == nil || c.User.Username != "mdxinc" {
		t.Errorf("user = %+v", c.User)
	}
	if c.InspirationSlug != "hero-tower-garage-doors" {
		t.Errorf("inspiration slug = %q", c.InspirationSlug)
	}
	if c.ExternalURL != "http://towerdoors.com.au" {
		t.Errorf("external url = %q", c.ExternalURL)
	}
}

func TestParseCardsFallbackScan(t *testing.T) {
	// Redesign-survival path: card attribute present but no js-collectable <li>
	// wrapper. The bare attribute scan must still find the card.
	doc := `<div class="future-card" data-collectable-model-value="{&quot;id&quot;:1,&quot;slug&quot;:&quot;future&quot;,&quot;title&quot;:&quot;Future&quot;,&quot;createdAt&quot;:1700000000,&quot;tags&quot;:[&quot;Clean&quot;],&quot;type&quot;:&quot;submission&quot;}"></div>`
	cards := ParseCards(doc)
	if len(cards) != 1 || cards[0].Slug != "future" || cards[0].Tags[0] != "Clean" {
		t.Fatalf("fallback scan cards = %+v", cards)
	}
}

func TestParentSlugForElement(t *testing.T) {
	tests := []struct {
		insp, typ, want string
	}{
		{"hero-tower-garage-doors", "hero", "tower-garage-doors"},
		{"404-page-monolog", "404_page", "monolog"},
		{"footer-bucks-sauce", "footer", "bucks-sauce"},
		{"unrelated-slug", "hero", ""},
		{"", "hero", ""},
	}
	for _, tt := range tests {
		if got := ParentSlugForElement(tt.insp, tt.typ); got != tt.want {
			t.Errorf("ParentSlugForElement(%q, %q) = %q, want %q", tt.insp, tt.typ, got, tt.want)
		}
	}
}

func TestParseDetail(t *testing.T) {
	d := ParseDetail("monolog", detailFixture)
	if d.Title != "MONOLOG" || d.ExternalURL != "https://bymonolog.com/" {
		t.Errorf("title/external = %q/%q", d.Title, d.ExternalURL)
	}
	if d.Award != "SOTD" {
		t.Errorf("award = %q", d.Award)
	}
	want := Scores{Design: 7.61, Usability: 7.29, Creativity: 7.2, Content: 7.54, Overall: 7.43}
	if d.Scores != want {
		t.Errorf("scores = %+v, want %+v", d.Scores, want)
	}
	if len(d.Jury) != 1 {
		t.Fatalf("jury = %d entries", len(d.Jury))
	}
	j := d.Jury[0]
	if j.Name != "Siarhiej Lisouski" || j.Country != "Georgia" || j.Profile != "lisovskiy" {
		t.Errorf("juror = %+v", j)
	}
	if len(j.Scores) != 5 || j.Scores[4] != 7.75 {
		t.Errorf("juror scores = %v", j.Scores)
	}
	if len(d.Palette) != 2 || d.Palette[0] != "#080807" || d.Palette[1] != "#DDDDD5" {
		t.Errorf("palette = %v", d.Palette)
	}
	if len(d.Tags) != 2 || d.Tags[1].Slug != "gsap" || d.Tags[1].Label != "GSAP" {
		t.Errorf("tags = %v", d.Tags)
	}
	if len(d.Credits) != 2 || d.Credits[0].Username != "byhuy" || d.Credits[0].DisplayName != "Huy Nguyen" {
		t.Errorf("credits = %v", d.Credits)
	}
}

func TestParseDetailNomineeVotes(t *testing.T) {
	// Nominee/honorable-mention pages render votes with the name outside the
	// anchor and `<i>from</i>` for the country.
	doc := `<title>FURO - Awwwards Honorable Mention</title>
<li class="js-hidden-list-element list-jury-notes__item"> <div class="list-jury-notes__info"> <figure> <a href="/byappi/"> <img src="x.png" /> </a> </figure> <div class="info"> <div> <strong>AP Studio</strong> <i>from</i> <strong>United States</strong> </div> <div> byappi.com </div> </div> </div> <div class="list-jury-notes__score"> <div class="grid-score" style="--score-cols: 5"> <div class="grid-score__item">9</div> <div class="grid-score__item">8</div> <div class="grid-score__item">7</div> <div class="grid-score__item">8</div> <div class="grid-score__item grid-score__item--total">8.20</div> </div> </div> </li>`
	d := ParseDetail("furo-web", doc)
	if d.Award != "Honorable Mention" {
		t.Errorf("award = %q", d.Award)
	}
	if len(d.Jury) != 1 {
		t.Fatalf("votes = %d", len(d.Jury))
	}
	v := d.Jury[0]
	if v.Name != "AP Studio" || v.Country != "United States" || v.Profile != "byappi" {
		t.Errorf("vote = %+v", v)
	}
	if len(v.Scores) != 5 || v.Scores[4] != 8.2 {
		t.Errorf("vote scores = %v", v.Scores)
	}
}

func TestParseDetailWeightedOverallFallback(t *testing.T) {
	// No js-chart-bar note: overall computes from the official weighting.
	doc := `<title>X - Awwwards Nominee</title>
<div class="layout-overall__score"><strong>8 / 10</strong></div>
<div class="layout-overall__score"><strong>8 / 10</strong></div>
<div class="layout-overall__score"><strong>8 / 10</strong></div>
<div class="layout-overall__score"><strong>8 / 10</strong></div>`
	d := ParseDetail("x", doc)
	if d.Scores.Overall != 8 {
		t.Errorf("weighted overall = %v, want 8", d.Scores.Overall)
	}
	if d.Award != "Nominee" {
		t.Errorf("award = %q", d.Award)
	}
}

func TestTagFilterSlug(t *testing.T) {
	tests := []struct{ in, want string }{
		{"Design Agencies", "design-agencies"},
		{"Three.js", "three-js"},
		{"GSAP", "gsap"},
		{"Art & Illustration", "art-and-illustration"},
	}
	for _, tt := range tests {
		if got := TagFilterSlug(tt.in); got != tt.want {
			t.Errorf("TagFilterSlug(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestIsTech(t *testing.T) {
	for _, tech := range []string{"GSAP", "Three.js", "Webflow", "webgl"} {
		if !IsTech(tech) {
			t.Errorf("IsTech(%q) = false, want true", tech)
		}
	}
	for _, not := range []string{"Clean", "Design Agencies", "Typography"} {
		if IsTech(not) {
			t.Errorf("IsTech(%q) = true, want false", not)
		}
	}
}
