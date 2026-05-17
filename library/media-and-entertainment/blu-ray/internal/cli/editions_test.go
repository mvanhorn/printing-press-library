package cli

// PATCH: Regression coverage for scoping umbrella edition parsing.

import "testing"

func TestParseEditionsHTMLScopesToContentOverview(t *testing.T) {
	t.Parallel()

	body := []byte(`<html><body>
		<aside><a href="https://www.blu-ray.com/movies/Sidebar-Blu-ray/111/">Sidebar</a> US $99.99</aside>
		<div id="content_overview">
			<table class="menu"><tr>
				<td><a href="https://www.blu-ray.com/movies/Inside-4K-Blu-ray/222/">Inside Edition</a></td>
				<td>UK $.99 $14.99</td>
			</tr></table>
		</div>
	</body></html>`)

	rows, err := parseEditionsHTML(body, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows len = %d, want 1: %+v", len(rows), rows)
	}
	if rows[0].ID != 222 {
		t.Fatalf("parsed id = %d, want inside edition id 222", rows[0].ID)
	}
	if rows[0].CurrentPrice != 0.99 {
		t.Fatalf("current price = %v, want 0.99", rows[0].CurrentPrice)
	}
}
