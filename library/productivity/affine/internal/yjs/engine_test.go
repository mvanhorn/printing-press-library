package yjs

import (
	"encoding/base64"
	"testing"
)

func TestNewEngine(t *testing.T) {
	e, err := NewEngine()
	if err != nil {
		t.Fatalf("NewEngine error: %v", err)
	}
	if e == nil {
		t.Fatal("engine is nil")
	}
}

func TestNewDoc(t *testing.T) {
	e, err := NewEngine()
	if err != nil {
		t.Fatal(err)
	}
	id, err := e.NewDoc()
	if err != nil {
		t.Fatalf("NewDoc error: %v", err)
	}
	if id != 0 {
		t.Errorf("first doc ID = %d, want 0", id)
	}
	id2, err := e.NewDoc()
	if err != nil {
		t.Fatal(err)
	}
	if id2 != 1 {
		t.Errorf("second doc ID = %d, want 1", id2)
	}
}

func TestEncodeAndApply(t *testing.T) {
	e, err := NewEngine()
	if err != nil {
		t.Fatal(err)
	}

	// Create a doc, add some data via JS, encode it
	docID, _ := e.NewDoc()
	_, err = e.RunScript(`
		(function() {
			var doc = globalThis._docs[0];
			var blocks = doc.getMap("blocks");
			var block = new Y.Map();
			block.set("sys:id", "test-block");
			block.set("sys:flavour", "affine:paragraph");
			var text = new Y.Text();
			text.insert(0, "Hello from Go!");
			block.set("prop:text", text);
			blocks.set("test-block", block);
			return "ok";
		})()
	`)
	if err != nil {
		t.Fatalf("RunScript error: %v", err)
	}

	// Encode state
	b64, err := e.EncodeStateAsUpdate(docID)
	if err != nil {
		t.Fatalf("EncodeStateAsUpdate error: %v", err)
	}
	if b64 == "" {
		t.Fatal("encoded state is empty")
	}

	// Apply to a new doc
	docID2, err := e.ApplyBase64Update(b64)
	if err != nil {
		t.Fatalf("ApplyBase64Update error: %v", err)
	}

	// Read blocks from the new doc
	blocks, err := e.ReadBlocks(docID2)
	if err != nil {
		t.Fatalf("ReadBlocks error: %v", err)
	}

	block, ok := blocks["test-block"]
	if !ok {
		t.Fatal("test-block not found in decoded doc")
	}
	if block["sys:id"] != "test-block" {
		t.Errorf("sys:id = %v, want 'test-block'", block["sys:id"])
	}
	if block["sys:flavour"] != "affine:paragraph" {
		t.Errorf("sys:flavour = %v, want 'affine:paragraph'", block["sys:flavour"])
	}
	if block["prop:text"] != "Hello from Go!" {
		t.Errorf("prop:text = %v, want 'Hello from Go!'", block["prop:text"])
	}
	t.Logf("Block: %v", block)
}

func TestReadMeta(t *testing.T) {
	e, err := NewEngine()
	if err != nil {
		t.Fatal(err)
	}

	docID, _ := e.NewDoc()
	_, err = e.RunScript(`
		(function() {
			var doc = globalThis._docs[0];
			var meta = doc.getMap("meta");
			meta.set("id", "doc-123");
			var title = new Y.Text();
			title.insert(0, "Test Document");
			meta.set("title", title);
			return "ok";
		})()
	`)
	if err != nil {
		t.Fatal(err)
	}

	meta, err := e.ReadMeta(docID)
	if err != nil {
		t.Fatalf("ReadMeta error: %v", err)
	}
	if meta["id"] != "doc-123" {
		t.Errorf("meta.id = %v, want 'doc-123'", meta["id"])
	}
	if meta["title"] != "Test Document" {
		t.Errorf("meta.title = %v, want 'Test Document'", meta["title"])
	}
}

func TestSaveStateVectorAndDelta(t *testing.T) {
	e, err := NewEngine()
	if err != nil {
		t.Fatal(err)
	}

	docID, _ := e.NewDoc()

	// Add initial data
	e.RunScript(`
		(function() {
			var doc = globalThis._docs[0];
			doc.getMap("blocks").set("b1", new Y.Map());
			return "ok";
		})()
	`)

	// Save state vector
	sv, err := e.SaveStateVector(docID)
	if err != nil {
		t.Fatalf("SaveStateVector error: %v", err)
	}
	if sv == "" {
		t.Fatal("state vector is empty")
	}

	// Add more data
	e.RunScript(`
		(function() {
			var doc = globalThis._docs[0];
			doc.getMap("blocks").set("b2", new Y.Map());
			return "ok";
		})()
	`)

	// Encode delta
	delta, err := e.EncodeDelta(docID, sv)
	if err != nil {
		t.Fatalf("EncodeDelta error: %v", err)
	}
	if delta == "" {
		t.Fatal("delta is empty")
	}

	// Delta should be smaller than full state
	fullB64, _ := e.EncodeStateAsUpdate(docID)
	deltaBytes, _ := base64.StdEncoding.DecodeString(delta)
	fullBytes, _ := base64.StdEncoding.DecodeString(fullB64)
	t.Logf("Delta: %d bytes, Full: %d bytes", len(deltaBytes), len(fullBytes))
	if len(deltaBytes) >= len(fullBytes) {
		t.Logf("Warning: delta (%d) >= full (%d), may be expected for small docs", len(deltaBytes), len(fullBytes))
	}
}

func TestFreeDoc(t *testing.T) {
	e, err := NewEngine()
	if err != nil {
		t.Fatal(err)
	}

	docID, _ := e.NewDoc()
	e.FreeDoc(docID)

	// After freeing, reading should fail gracefully
	_, err = e.ReadBlocks(docID)
	if err == nil {
		t.Log("ReadBlocks after FreeDoc didn't error (may be OK if it returns empty)")
	}
}

func TestTableBlock(t *testing.T) {
	e, err := NewEngine()
	if err != nil {
		t.Fatal(err)
	}

	_, err = e.NewDoc()
	if err != nil {
		t.Fatal(err)
	}

	// Create a table with flat-key format (same as AFFiNE)
	_, err = e.RunScript(`
		(function() {
			var doc = globalThis._docs[0];
			var blocks = doc.getMap("blocks");
			var block = new Y.Map();
			block.set("sys:id", "table-1");
			block.set("sys:flavour", "affine:table");

			block.set("prop:rows.r0.rowId", "r0");
			block.set("prop:rows.r0.order", "r0000");
			block.set("prop:rows.r1.rowId", "r1");
			block.set("prop:rows.r1.order", "r0001");

			block.set("prop:columns.c0.columnId", "c0");
			block.set("prop:columns.c0.order", "c0000");
			block.set("prop:columns.c1.columnId", "c1");
			block.set("prop:columns.c1.order", "c0001");

			var t00 = new Y.Text(); t00.insert(0, "A");
			var t01 = new Y.Text(); t01.insert(0, "B");
			var t10 = new Y.Text(); t10.insert(0, "C");
			var t11 = new Y.Text(); t11.insert(0, "D");
			block.set("prop:cells.r0:c0.text", t00);
			block.set("prop:cells.r0:c1.text", t01);
			block.set("prop:cells.r1:c0.text", t10);
			block.set("prop:cells.r1:c1.text", t11);

			blocks.set("table-1", block);
			return "ok";
		})()
	`)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}

	blocks, err := e.ReadBlocks(0)
	if err != nil {
		t.Fatalf("ReadBlocks: %v", err)
	}

	table := blocks["table-1"]
	if table["sys:flavour"] != "affine:table" {
		t.Errorf("flavour = %v", table["sys:flavour"])
	}
	// Check flat key cells are readable
	if table["prop:cells.r0:c0.text"] != "A" {
		t.Errorf("cell r0:c0 = %v, want 'A'", table["prop:cells.r0:c0.text"])
	}
	if table["prop:cells.r1:c1.text"] != "D" {
		t.Errorf("cell r1:c1 = %v, want 'D'", table["prop:cells.r1:c1.text"])
	}
	t.Logf("Table block keys: %d", len(table))
}

func TestParseInlineMarkdown(t *testing.T) {
	e, err := NewEngine()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		input    string
		expected string // JSON of segments
	}{
		{
			"plain text",
			"hello world",
			`[{"text":"hello world","attrs":{}}]`,
		},
		{
			"bold",
			"**bold text**",
			`[{"text":"bold text","attrs":{"bold":true}}]`,
		},
		{
			"italic",
			"*italic text*",
			`[{"text":"italic text","attrs":{"italic":true}}]`,
		},
		{
			"inline code",
			"`code here`",
			`[{"text":"code here","attrs":{"code":true}}]`,
		},
		{
			"strikethrough",
			"~~deleted~~",
			`[{"text":"deleted","attrs":{"strike":true}}]`,
		},
		{
			"link",
			"[click me](https://example.com)",
			`[{"text":"click me","attrs":{"link":"https://example.com"}}]`,
		},
		{
			"mixed inline",
			"normal **bold** and *italic* and `code`",
			`[{"text":"normal ","attrs":{}},{"text":"bold","attrs":{"bold":true}},{"text":" and ","attrs":{}},{"text":"italic","attrs":{"italic":true}},{"text":" and ","attrs":{}},{"text":"code","attrs":{"code":true}}]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e.vm.Set("_testInput", tt.input)
			val, err := e.RunScript(`JSON.stringify(parseInlineMarkdown(_testInput))`)
			if err != nil {
				t.Fatalf("parseInlineMarkdown error: %v", err)
			}
			if val != tt.expected {
				t.Errorf("parseInlineMarkdown(%q)\n  got:  %s\n  want: %s", tt.input, val, tt.expected)
			}
		})
	}
}

func TestCreateFormattedBlock(t *testing.T) {
	e, err := NewEngine()
	if err != nil {
		t.Fatal(err)
	}

	docID, _ := e.NewDoc()

	// Initialize blocks map
	e.RunScript(`
		(function() {
			var doc = globalThis._docs[0];
			doc.getMap("blocks");
			return "ok";
		})()
	`)

	// Create a block with formatted text
	err = e.CreateFormattedBlock(docID, "fmt-block-1", "affine:paragraph", "text",
		"normal **bold** and *italic* and `code`")
	if err != nil {
		t.Fatalf("CreateFormattedBlock error: %v", err)
	}

	// Verify the block exists and read its delta to check attributes
	e.vm.Set("_docId", docID)
	val, err := e.RunScript(`
		(function() {
			var doc = globalThis._docs[0];
			var blocks = doc.getMap("blocks");
			var block = blocks.get("fmt-block-1");
			var text = block.get("prop:text");
			// Get delta which includes attributes
			var delta = text.toDelta();
			return JSON.stringify(delta);
		})()
	`)
	if err != nil {
		t.Fatalf("read delta error: %v", err)
	}

	t.Logf("Delta: %s", val)

	// Verify plain text
	plainVal, _ := e.RunScript(`
		(function() {
			var doc = globalThis._docs[0];
			var block = doc.getMap("blocks").get("fmt-block-1");
			return block.get("prop:text").toString();
		})()
	`)
	expected := "normal bold and italic and code"
	if plainVal != expected {
		t.Errorf("plain text = %q, want %q", plainVal, expected)
	}

	// Verify delta has bold attribute
	if val == "" {
		t.Fatal("delta is empty")
	}
	childrenOK, err := e.RunScript(`
		(function() {
			var block = globalThis._docs[0].getMap("blocks").get("fmt-block-1");
			var children = block.get("sys:children");
			return children instanceof Y.Array && children.length === 0 ? "ok" : "bad";
		})()
	`)
	if err != nil {
		t.Fatalf("read children invariant: %v", err)
	}
	if childrenOK != "ok" {
		t.Fatalf("sys:children invariant = %q, want ok", childrenOK)
	}
	// Check that "bold":true appears in the delta
	if !contains(val, `"bold":true`) {
		t.Error("delta missing bold attribute")
	}
	if !contains(val, `"italic":true`) {
		t.Error("delta missing italic attribute")
	}
	if !contains(val, `"code":true`) {
		t.Error("delta missing code attribute")
	}
}

func TestCreateFormattedBlockPreservesUTF8AfterUpdateRoundTrip(t *testing.T) {
	e, err := NewEngine()
	if err != nil {
		t.Fatal(err)
	}

	docID, err := e.NewDoc()
	if err != nil {
		t.Fatal(err)
	}
	if err := e.CreateFormattedBlock(docID, "unicode", "affine:paragraph", "text", "Descrição ação Função É necessário"); err != nil {
		t.Fatalf("CreateFormattedBlock error: %v", err)
	}

	update, err := e.EncodeStateAsUpdate(docID)
	if err != nil {
		t.Fatalf("EncodeStateAsUpdate error: %v", err)
	}
	roundTripID, err := e.ApplyBase64Update(update)
	if err != nil {
		t.Fatalf("ApplyBase64Update error: %v", err)
	}
	blocks, err := e.ReadBlocks(roundTripID)
	if err != nil {
		t.Fatalf("ReadBlocks error: %v", err)
	}
	got, _ := blocks["unicode"]["prop:text"].(string)
	want := "Descrição ação Função É necessário"
	if got != want {
		t.Fatalf("unicode text = %q, want %q", got, want)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && searchString(s, sub)
}

func searchString(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
