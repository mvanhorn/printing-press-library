package cli

import "testing"

func TestApplyBrowseMetadata_ExtractsTotalSortsAndRefinements(t *testing.T) {
	t.Parallel()

	body := []byte(`
<div class="row search-results-wrap" data-gtm-results="1172"></div>
<div class="collapsible-lg pwc-border--radius active refinement refinement-food.sugarfree" data-refinement-type="food.SugarFree" >
  <span class="refinement-title-text">Sem Adição de Açúcar</span>
  <div class="content value refinement-content " id="refinement-sem-adicao-de-acucar">
    <ul class="values content row">
      <li class="refinement-value-block col-sm-6 col-md-12">
        <ul>
          <li class="refinement-value">
            <button data-href="/on/demandware.store/Sites-continente-Site/default/Search-ShowAjax?cgid=col-produtos&amp;q=leite&amp;pmin=0%2e01&amp;prefn1=food%2eSugarFree&amp;prefv1=Produto%20Sem%20Adi%c3%a7%c3%a3o%20de%20A%c3%a7%c3%bacar" class="refinement-btn " >
              <div class="pwc-form-checkbox col-form-checkbox">
                <input id="Produto Sem Adição de Açúcar" type="checkbox" title  />
                <label class="pwc-form-check-label" for="Produto Sem Adição de Açúcar">
                  Produto Sem Adição de Açúcar <span class="hit-count">(170)</span>
                </label>
              </div>
            </button>
          </li>
        </ul>
      </li>
    </ul>
  </div>
</div>
<div class="collapsible-lg pwc-border--radius active refinement refinement-category" data-refinement-type="category" >
  <span class="refinement-title-text">Categorias</span>
  <button class="category-refinement-btn " data-cgid="laticinios" data-href="/on/demandware.store/Sites-continente-Site/default/Search-ShowAjax?q=leite&amp;cgid=laticinios">
    <span title="Laticínios" class="js-category-name-item">Laticínios <span class="hit-count">(44)</span></span>
  </button>
</div>
<div data-sort-options="{&quot;options&quot;:[{&quot;displayName&quot;:&quot;Relev&acirc;ncia&quot;,&quot;id&quot;:&quot;search-relevance&quot;,&quot;url&quot;:&quot;https://www.continente.pt/on/demandware.store/Sites-continente-Site/default/Search-UpdateGrid?q=leite&amp;srule=Continente&amp;sz=5&quot;}],&quot;ruleId&quot;:&quot;Continente&quot;}"></div>
`)

	resp := searchResponse{}
	applyBrowseMetadata(&resp, body)

	if resp.TotalCount != 1172 {
		t.Fatalf("total count = %d, want 1172", resp.TotalCount)
	}
	if len(resp.SortOptions) != 1 || resp.SortOptions[0].DisplayName != "Relevância" {
		t.Fatalf("unexpected sort options: %+v", resp.SortOptions)
	}
	if len(resp.Refinements) == 0 {
		t.Fatalf("expected at least one refinement, got none")
	}
	var sawCategory bool
	for _, refinement := range resp.Refinements {
		switch refinement.Key {
		case "category":
			sawCategory = true
			if refinement.Options[0].CategoryID != "laticinios" {
				t.Fatalf("category option = %+v", refinement.Options[0])
			}
		}
	}
	if !sawCategory {
		t.Fatalf("missing expected refinements: %+v", resp.Refinements)
	}
}
