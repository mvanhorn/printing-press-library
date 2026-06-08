package cli

import (
	"encoding/json"
	"testing"
)

func TestParseMiniCartHTML(t *testing.T) {
	t.Parallel()

	raw := []byte(`
<a class="minicart-link" href="https://www.continente.pt/recomendacoes/?rurl=3"></a>
<span class="minicart-quantity ">1</span>
<span class="minicart-grandtotal">2,49&euro;</span>
<span class="d-none js-minicart-actions" data-add-to-cart="https://www.continente.pt/on/demandware.store/Sites-continente-Site/default/Cart-AddProduct" data-remove-action="https://www.continente.pt/on/demandware.store/Sites-continente-Site/default/Cart-RemoveProductLineItem" data-action="https://www.continente.pt/on/demandware.store/Sites-continente-Site/default/Cart-UpdateQuantity"></span>
<div class="ct-popover" data-productqtymap="{&quot;items&quot;:[{&quot;id&quot;:&quot;5119481&quot;,&quot;quantity&quot;:1}]}"></div>
`)

	got, err := parseMiniCartHTML(raw)
	if err != nil {
		t.Fatalf("parseMiniCartHTML: %v", err)
	}
	if got.Quantity != 1 {
		t.Fatalf("Quantity = %d, want 1", got.Quantity)
	}
	if got.GrandTotal != "2,49€" {
		t.Fatalf("GrandTotal = %q, want %q", got.GrandTotal, "2,49€")
	}
	if got.CartURL != "https://www.continente.pt/recomendacoes/?rurl=3" {
		t.Fatalf("CartURL = %q", got.CartURL)
	}
	if got.Actions["add"] == "" || got.Actions["remove"] == "" || got.Actions["update_quantity"] == "" {
		t.Fatalf("Actions not parsed: %#v", got.Actions)
	}
}

func TestParseMiniCartShowJSON(t *testing.T) {
	t.Parallel()

	raw := []byte(`{
  "quantityTotal": 3,
  "basket": {
    "itemsSortedByBrand": [
      {
        "items": [
          {
            "id": "8061027",
            "productName": "Agua",
            "brand": "Pedras",
            "uuid": "entry-1",
            "UUID": "line-1",
            "selectedDimension": "un",
            "secondaryQuantity": 2,
            "productURL": "https://www.continente.pt/produto/agua.html",
            "price": {
              "sales": {
                "formatted": "4,39€"
              }
            }
          },
          {
            "id": "7127340",
            "productName": "Saco",
            "brand": "Continente",
            "uuid": "entry-2",
            "UUID": "line-2",
            "selectedDimension": "un",
            "secondaryQuantity": 1,
            "productURL": "https://www.continente.pt/produto/saco.html",
            "price": {
              "sales": {
                "formatted": "0,10€"
              }
            }
          }
        ]
      }
    ]
  }
}`)

	got, err := parseMiniCartShowJSON(raw)
	if err != nil {
		t.Fatalf("parseMiniCartShowJSON: %v", err)
	}
	if got.Quantity != 3 {
		t.Fatalf("Quantity = %d, want 3", got.Quantity)
	}
	if len(got.Items) != 2 {
		t.Fatalf("len(Items) = %d, want 2", len(got.Items))
	}
	if got.Items[0].UUID != "line-1" || got.Items[0].EntryUUID != "entry-1" {
		t.Fatalf("unexpected first item UUIDs: %#v", got.Items[0])
	}
	if got.Items[1].Price != "0,10€" {
		t.Fatalf("unexpected second item price: %#v", got.Items[1])
	}
}

func TestCompactMiniCartPayloadDropsVerboseFields(t *testing.T) {
	t.Parallel()

	payload := miniCartPayload{
		Quantity:   3,
		GrandTotal: "4,49€",
		CartURL:    "https://www.continente.pt/checkout/carrinho/",
		Actions: map[string]string{
			"add": "x",
		},
		ProductQtyMap: map[string]any{
			"items": []any{map[string]any{"id": "8061027"}},
		},
		Items: []cartLineItem{{
			ProductID:  "8061027",
			Name:       "Agua",
			Brand:      "Pedras",
			Quantity:   2,
			UUID:       "line-1",
			EntryUUID:  "entry-1",
			Dimension:  "un",
			ProductURL: "https://www.continente.pt/produto/agua.html",
			Price:      "4,39€",
		}},
	}

	compact := compactMiniCartPayload(payload)
	if compact.Actions != nil {
		t.Fatalf("compact actions should be nil: %#v", compact.Actions)
	}
	if compact.ProductQtyMap != nil {
		t.Fatalf("compact product qty map should be nil: %#v", compact.ProductQtyMap)
	}
	if got := compact.Items[0].EntryUUID; got != "" {
		t.Fatalf("compact entry uuid = %q, want empty", got)
	}
	if got := compact.Items[0].ProductURL; got != "" {
		t.Fatalf("compact product url = %q, want empty", got)
	}
}

func TestCartMutationSummaryCompactShape(t *testing.T) {
	t.Parallel()

	summary := cartMutationSummary{
		Action:    "update",
		ProductID: "8061027",
		UUID:      "line-1",
		Quantity:  2,
		Cart: compactMiniCartPayload(miniCartPayload{
			Quantity:   2,
			GrandTotal: "4,39€",
			Items: []cartLineItem{{
				ProductID: "8061027",
				Name:      "Agua",
				Quantity:  2,
				UUID:      "line-1",
				Price:     "4,39€",
			}},
		}),
	}

	raw, err := json.Marshal(summary)
	if err != nil {
		t.Fatalf("json.Marshal(summary): %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("json.Unmarshal(summary): %v", err)
	}
	if _, ok := decoded["cart"]; !ok {
		t.Fatalf("compact mutation summary missing cart: %#v", decoded)
	}
}
