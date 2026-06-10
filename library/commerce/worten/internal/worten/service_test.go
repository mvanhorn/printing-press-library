package worten

import "testing"

func TestNormalizeBuyerViewDishwasher(t *testing.T) {
	details := map[string]any{
		"productsData": map[string]any{
			"products": []any{
				map[string]any{
					"id": "9e55ed12-bc3f-40a6-98b6-d849c9814502",
					"meta": map[string]any{
						"refs": map[string]any{
							"sku":           "7407993",
							"entity_id":     "1071697237",
							"iron_model_id": "1002240862",
							"ean":           []any{"7332543809790"},
						},
					},
					"woffer": map[string]any{"offer_id": "offer-1"},
					"properties": map[string]any{
						"text": map[string]any{
							"name":               "Máquina de Lavar Loiça Encastre AEG FSE74707P (15 Conjuntos - 60 cm - Painel Preto)",
							"brand_name":         "AEG",
							"long_description":   "Capacidade para 15 conjuntos. Nível de ruído de 42 db.",
							"energy-class-new":   "C",
						},
					},
					"image": map[string]any{"transforms": map[string]any{"default": "image-path"}},
					"labels": map[string]any{"badges": []any{}, "categories": []any{"CMS-1"}},
					"ratings": map[string]any{"val": 4.6, "cnt": 10},
				},
			},
		},
		"offersData": map[string]any{
			"offers": []any{
				map[string]any{
					"offer_id":  "offer-1",
					"seller_id": "seller-1",
					"price": map[string]any{
						"original": "1049",
						"final":    "1049",
						"ptype":    "none",
					},
					"stock": map[string]any{"is_in_stock": false},
					"shipping": map[string]any{
						"price":       "0",
						"lead_time":   0,
						"safety_time": 0,
						"max_time":    0,
						"min_time":    0,
					},
				},
			},
		},
		"sellersData": []any{
			map[string]any{
				"seller_id":     "seller-1",
				"name":          "Worten",
				"premium_state": "",
				"rating":        map[string]any{"evaluations": 0, "rating": "0"},
				"status":        map[string]any{"orders": 0},
			},
		},
		"productsCanonicalsData": map[string]any{
			"web_items": []any{
				map[string]any{"url": "/produtos/maquina-de-lavar-loica-encastre-aeg-fse74707p-15-conjuntos-60-cm-painel-preto-7407993"},
			},
		},
		"features": map[string]any{"show_free_delivery_installation_pickup": true},
	}
	specs := map[string]any{
		"sections": []any{
			map[string]any{
				"title": "Referências",
				"rows": []any{
					map[string]any{"subtitle": "Modelo", "specs": "FSE74707P"},
				},
			},
			map[string]any{
				"title": "Características Físicas",
				"rows": []any{
					map[string]any{"subtitle": "Altura", "specs": "82-90"},
					map[string]any{"subtitle": "Largura", "specs": "60"},
					map[string]any{"subtitle": "Profundidade", "specs": "57"},
				},
			},
			map[string]any{
				"title": "Características Específicas",
				"rows": []any{
					map[string]any{"subtitle": "Capacidade", "specs": "15"},
					map[string]any{"subtitle": "Instalação", "specs": "Painel Oculto"},
					map[string]any{"subtitle": "Tipo de Secagem", "specs": "Sensorlogic"},
				},
			},
			map[string]any{
				"title": "Eficiência Energética",
				"rows": []any{
					map[string]any{"subtitle": "Nível de ruído db", "specs": "42"},
					map[string]any{"subtitle": "Consumo água l/ciclo", "specs": "11"},
					map[string]any{"subtitle": "Consumo de energia", "specs": "76 kWh / 100 ciclos"},
				},
			},
			map[string]any{
				"title": "Extras",
				"rows": []any{
					map[string]any{"subtitle": "3º Nível para Talheres", "specs": "Sim"},
					map[string]any{"subtitle": "Cesto Regulável em Altura", "specs": "Sim"},
					map[string]any{"subtitle": "Conectividade", "specs": "Não"},
				},
			},
		},
	}

	buyer, err := normalizeBuyerView(details, specs)
	if err != nil {
		t.Fatalf("normalizeBuyerView returned error: %v", err)
	}
	if buyer["category"] != "dishwasher" {
		t.Fatalf("expected dishwasher category, got %v", buyer["category"])
	}
	if buyer["soldByWorten"] != true {
		t.Fatalf("expected soldByWorten true, got %v", buyer["soldByWorten"])
	}
	if buyer["capacitySets"] != float64(15) {
		t.Fatalf("expected capacitySets 15, got %#v", buyer["capacitySets"])
	}
	if buyer["dryingType"] != "Sensorlogic" {
		t.Fatalf("expected dryingType Sensorlogic, got %#v", buyer["dryingType"])
	}
	if buyer["thirdCutleryLevel"] != true {
		t.Fatalf("expected thirdCutleryLevel true, got %#v", buyer["thirdCutleryLevel"])
	}
	if buyer["energyPer100CyclesKwh"] != float64(76) {
		t.Fatalf("expected energyPer100CyclesKwh 76, got %#v", buyer["energyPer100CyclesKwh"])
	}
}

func TestFindCachedProductIDMatchesCanonicalURL(t *testing.T) {
	cache := map[string]any{
		"products": map[string]any{
			"https://www.worten.pt/produtos/example": map[string]any{
				"productId":    "11111111-1111-1111-1111-111111111111",
				"canonicalUrl": "https://www.worten.pt/produtos/example?bvstate=foo",
			},
		},
	}
	got := findCachedProductID(cache, "https://www.worten.pt/produtos/example?bvstate=bar")
	if got != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("expected cached product id, got %q", got)
	}
}

func TestNormalizeProductDetailsExposesStructuredStockContext(t *testing.T) {
	normalized, err := normalizeProductDetails(map[string]any{
		"features": map[string]any{
			"show_free_delivery_installation_pickup": true,
		},
		"productsCanonicalsData": map[string]any{
			"web_items": []any{map[string]any{"url": "/produtos/example-123"}},
		},
		"productsData": map[string]any{
			"products": []any{
				map[string]any{
					"id": "product-1",
					"meta": map[string]any{
						"refs": map[string]any{
							"sku":           "123",
							"entity_id":     "entity-1",
							"iron_model_id": "model-1",
							"ean":           []any{"ean-1"},
						},
					},
					"properties": map[string]any{
						"text": map[string]any{
							"name":       "Example product",
							"brand_name": "WORTEN",
						},
					},
					"image": map[string]any{
						"transforms": map[string]any{"default": "image-1"},
					},
					"woffer": map[string]any{"offer_id": "offer-1"},
				},
			},
		},
		"offersData": map[string]any{
			"offers": []any{
				map[string]any{
					"offer_id":  "offer-1",
					"seller_id": "seller-1",
					"stock":     map[string]any{"is_in_stock": true},
					"shipping": map[string]any{
						"price":       "0",
						"lead_time":   1,
						"min_time":    1,
						"max_time":    2,
						"safety_time": 1,
					},
					"price": map[string]any{"final": "99.99"},
				},
			},
		},
		"sellersData": []any{
			map[string]any{
				"seller_id":     "seller-1",
				"name":          "Worten",
				"premium_state": "",
				"rating":        map[string]any{"evaluations": 0, "rating": "0"},
				"status":        map[string]any{"orders": 0},
			},
		},
		"storePricingData": map[string]any{
			"storeId":   "store-1",
			"storeName": "Worten Vasco da Gama",
			"favoriteStores": []any{
				map[string]any{"id": "store-1", "name": "Worten Vasco da Gama", "city": "Lisboa"},
			},
			"retekSkus": []any{"123"},
		},
	})
	if err != nil {
		t.Fatalf("normalizeProductDetails returned error: %v", err)
	}
	stock, ok := normalized["stock"].(map[string]any)
	if !ok {
		t.Fatalf("expected stock object, got %#v", normalized["stock"])
	}
	localArea, ok := stock["localArea"].(map[string]any)
	if !ok {
		t.Fatalf("expected localArea object, got %#v", stock["localArea"])
	}
	signals, ok := stock["signals"].(map[string]any)
	if !ok {
		t.Fatalf("expected signals object, got %#v", stock["signals"])
	}
	if stock["available"] != true {
		t.Fatalf("expected available true, got %#v", stock["available"])
	}
	if localArea["nearLisbon"] != true {
		t.Fatalf("expected nearLisbon true, got %#v", localArea["nearLisbon"])
	}
	if signals["storePricingAvailable"] != true {
		t.Fatalf("expected storePricingAvailable true, got %#v", signals["storePricingAvailable"])
	}
}

func TestNormalizeStockWithoutLocalStoreDataStaysExplicit(t *testing.T) {
	stock := normalizeStock(
		map[string]any{
			"features": map[string]any{
				"show_free_delivery_installation_pickup": true,
			},
			"storePricingData": nil,
		},
		map[string]any{
			"stock": map[string]any{"is_in_stock": false},
			"shipping": map[string]any{
				"price":       "0",
				"lead_time":   4,
				"min_time":    4,
				"max_time":    6,
				"safety_time": 2,
			},
		},
		map[string]any{"name": "KIBO"},
		nil,
	)
	if stock["available"] != false {
		t.Fatalf("expected available false, got %#v", stock["available"])
	}
	if localArea := stock["localArea"]; localArea != nil {
		if typed, ok := localArea.(map[string]any); !ok || typed != nil {
			t.Fatalf("expected nil localArea, got %#v", stock["localArea"])
		}
	}
	signals := stock["signals"].(map[string]any)
	if signals["storePricingAvailable"] != false {
		t.Fatalf("expected storePricingAvailable false, got %#v", signals["storePricingAvailable"])
	}
	notes := stock["notes"].([]string)
	if len(notes) == 0 {
		t.Fatal("expected stock notes")
	}
}

func TestNormalizeOfferStoreSearchKeepsStructuredNearbyStores(t *testing.T) {
	storeSearch := normalizeOfferStoreSearch(map[string]any{
		"code":      "FOUND",
		"latitude":  "38.7679838",
		"longitude": "-9.0977209",
		"entries": []any{
			map[string]any{
				"store": map[string]any{
					"id":   "526",
					"name": "Worten Vasco da Gama",
					"address": map[string]any{
						"address":    "Centro Comercial Vasco da Gama",
						"city":       "Lisboa",
						"district":   "Lisboa",
						"postalCode": "1990-094",
					},
					"distance": 0.05907198769604151,
					"URL":      "/lojas/worten-vasco-gama-526",
					"features": []any{"PIS", "PIS_EXPRESS", "HD_EXPRESS"},
				},
				"status": "LOW",
			},
			map[string]any{
				"store": map[string]any{
					"id":   "1034",
					"name": "Worten Telheiras",
					"address": map[string]any{
						"address":    "Centro Comercial Continente Telheiras",
						"city":       "Lisboa",
						"district":   "Lisboa",
						"postalCode": "1600-528",
					},
					"distance": 6.774860804463862,
					"URL":      "/lojas/worten-telheiras-1034",
					"features": []any{"PIS", "PIS_EXPRESS"},
				},
				"status": "NONE",
			},
		},
	})
	if storeSearch["code"] != "FOUND" {
		t.Fatalf("expected code FOUND, got %#v", storeSearch["code"])
	}
	if storeSearch["storeCount"] != 2 {
		t.Fatalf("expected storeCount 2, got %#v", storeSearch["storeCount"])
	}
	if storeSearch["availableStoreCount"] != 1 {
		t.Fatalf("expected availableStoreCount 1, got %#v", storeSearch["availableStoreCount"])
	}
	if storeSearch["nearLisbon"] != true {
		t.Fatalf("expected nearLisbon true, got %#v", storeSearch["nearLisbon"])
	}
	stores := storeSearch["stores"].([]map[string]any)
	if stores[0]["status"] != "low" {
		t.Fatalf("expected first store status low, got %#v", stores[0]["status"])
	}
	if stores[0]["hasStock"] != true {
		t.Fatalf("expected first store hasStock true, got %#v", stores[0]["hasStock"])
	}
	if stores[1]["hasStock"] != false {
		t.Fatalf("expected second store hasStock false, got %#v", stores[1]["hasStock"])
	}
}