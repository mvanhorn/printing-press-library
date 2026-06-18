package yjs

import (
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"

	"github.com/dop251/goja"
)

//go:embed yjs_bundle.js
var yjsBundle string

// Engine wraps a goja JS runtime with Y.js loaded.
type Engine struct {
	vm *goja.Runtime
	mu sync.Mutex
}

// NewEngine creates a new Y.js engine with the bundled library loaded.
func NewEngine() (*Engine, error) {
	vm := goja.New()

	// Provide browser API shims that goja doesn't have
	vm.RunString(`
		var console = {
			log: function() {},
			warn: function() {},
			error: function() {},
			info: function() {},
			debug: function() {}
		};
		var crypto = {
			getRandomValues: function(arr) {
				for (var i = 0; i < arr.length; i++) {
					arr[i] = Math.floor(Math.random() * 256);
				}
				return arr;
			}
		};

		var _b64chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
		function btoa(str) {
			var out = "";
			for (var i = 0; i < str.length; i += 3) {
				var c1 = str.charCodeAt(i);
				var c2 = i + 1 < str.length ? str.charCodeAt(i + 1) : 0;
				var c3 = i + 2 < str.length ? str.charCodeAt(i + 2) : 0;
				out += _b64chars[(c1 >> 2) & 63];
				out += _b64chars[((c1 << 4) | (c2 >> 4)) & 63];
				out += i + 1 < str.length ? _b64chars[((c2 << 2) | (c3 >> 6)) & 63] : "=";
				out += i + 2 < str.length ? _b64chars[c3 & 63] : "=";
			}
			return out;
		}
		function atob(str) {
			str = str.replace(/=+$/, "");
			var out = "";
			for (var i = 0; i < str.length; i += 4) {
				var b1 = _b64chars.indexOf(str[i]);
				var b2 = i + 1 < str.length ? _b64chars.indexOf(str[i + 1]) : 0;
				var b3 = i + 2 < str.length ? _b64chars.indexOf(str[i + 2]) : 0;
				var b4 = i + 3 < str.length ? _b64chars.indexOf(str[i + 3]) : 0;
				out += String.fromCharCode(((b1 << 2) | (b2 >> 4)) & 255);
				if (i + 2 < str.length + (str.length % 4 ? 4 - str.length % 4 : 0)) out += String.fromCharCode(((b2 << 4) | (b3 >> 2)) & 255);
				if (i + 3 < str.length + (str.length % 4 ? 4 - str.length % 4 : 0)) out += String.fromCharCode(((b3 << 6) | b4) & 255);
			}
			return out;
		}
	`)

	// Load Y.js bundle
	_, err := vm.RunString(yjsBundle)
	if err != nil {
		return nil, fmt.Errorf("load yjs bundle: %w", err)
	}

	// Initialize docs array and inline markdown parser
	vm.RunString(`globalThis._docs = [];`)

	// Register inline markdown parser: converts **bold**, *italic*, ` +"`code`" + `, ~~strike~~, [text](url)
	// into segments with Y.Text-compatible attributes
	_, err = vm.RunString(`
		function parseInlineMarkdown(src) {
			var segments = [];
			var i = 0;
			var buf = "";

			function flush() {
				if (buf.length > 0) { segments.push({text: buf, attrs: {}}); buf = ""; }
			}

			while (i < src.length) {
				// Escaped character
				if (src[i] === '\\' && i + 1 < src.length) {
					buf += src[i + 1];
					i += 2;
					continue;
				}

				// Inline code: ` + "`...`" + `
				if (src[i] === '` + "`" + `') {
					var end = src.indexOf('` + "`" + `', i + 1);
					if (end !== -1) {
						flush();
						segments.push({text: src.substring(i + 1, end), attrs: {code: true}});
						i = end + 1;
						continue;
					}
				}

				// Bold: **...**
				if (src[i] === '*' && i + 1 < src.length && src[i + 1] === '*') {
					var end = src.indexOf('**', i + 2);
					if (end !== -1) {
						flush();
						// Recursively parse inner content for nested formatting
						var inner = src.substring(i + 2, end);
						var innerSegs = parseInlineMarkdown(inner);
						for (var j = 0; j < innerSegs.length; j++) {
							var s = innerSegs[j];
							var a = {}; for (var k in s.attrs) a[k] = s.attrs[k];
							a.bold = true;
							segments.push({text: s.text, attrs: a});
						}
						i = end + 2;
						continue;
					}
				}

				// Strikethrough: ~~...~~
				if (src[i] === '~' && i + 1 < src.length && src[i + 1] === '~') {
					var end = src.indexOf('~~', i + 2);
					if (end !== -1) {
						flush();
						var inner = src.substring(i + 2, end);
						var innerSegs = parseInlineMarkdown(inner);
						for (var j = 0; j < innerSegs.length; j++) {
							var s = innerSegs[j];
							var a = {}; for (var k in s.attrs) a[k] = s.attrs[k];
							a.strike = true;
							segments.push({text: s.text, attrs: a});
						}
						i = end + 2;
						continue;
					}
				}

				// Italic: *...*  (single asterisk, not followed by another)
				if (src[i] === '*' && (i + 1 >= src.length || src[i + 1] !== '*')) {
					var end = -1;
					for (var si = i + 1; si < src.length; si++) {
						if (src[si] === '*' && (si + 1 >= src.length || src[si + 1] !== '*') && (si === 0 || src[si - 1] !== '*')) {
							end = si; break;
						}
					}
					if (end !== -1) {
						flush();
						var inner = src.substring(i + 1, end);
						var innerSegs = parseInlineMarkdown(inner);
						for (var j = 0; j < innerSegs.length; j++) {
							var s = innerSegs[j];
							var a = {}; for (var k in s.attrs) a[k] = s.attrs[k];
							a.italic = true;
							segments.push({text: s.text, attrs: a});
						}
						i = end + 1;
						continue;
					}
				}

				// Link: [text](url)
				if (src[i] === '[') {
					var closeBracket = src.indexOf(']', i + 1);
					if (closeBracket !== -1 && closeBracket + 1 < src.length && src[closeBracket + 1] === '(') {
						var closeParen = src.indexOf(')', closeBracket + 2);
						if (closeParen !== -1) {
							flush();
							var linkText = src.substring(i + 1, closeBracket);
							var linkUrl = src.substring(closeBracket + 2, closeParen);
							segments.push({text: linkText, attrs: {link: linkUrl}});
							i = closeParen + 1;
							continue;
						}
					}
				}

				buf += src[i];
				i++;
			}
			flush();
			return segments;
		}
	`)
	if err != nil {
		return nil, fmt.Errorf("load markdown parser: %w", err)
	}

	return &Engine{vm: vm}, nil
}

// ApplyBase64Update creates a Y.Doc and applies a base64-encoded update.
// Returns the doc handle ID.
func (e *Engine) ApplyBase64Update(b64 string) (int, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return 0, fmt.Errorf("decode base64: %w", err)
	}

	e.vm.Set("_updateBytes", raw)
	val, err := e.vm.RunString(`
		(function() {
			var doc = new Y.Doc();
			var arr = new Uint8Array(_updateBytes);
			Y.applyUpdate(doc, arr);
			var id = globalThis._docs.length;
			globalThis._docs.push(doc);
			return id;
		})()
	`)
	if err != nil {
		return 0, fmt.Errorf("apply update: %w", err)
	}
	return int(val.ToInteger()), nil
}

// ReadBlocks reads all blocks from a Y.Doc and returns them as JSON-friendly map.
func (e *Engine) ReadBlocks(docID int) (map[string]map[string]any, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.vm.Set("_docId", docID)
	val, err := e.vm.RunString(`
		(function() {
			var doc = globalThis._docs[_docId];
			var blocks = doc.getMap("blocks");
			var result = {};
			blocks.forEach(function(block, blockId) {
				if (!(block instanceof Y.Map)) return;
				var b = {};
				block.forEach(function(v, k) {
					if (v instanceof Y.Text) b[k] = v.toString();
					else if (v instanceof Y.Array) {
						var arr = [];
						v.forEach(function(item) {
							if (typeof item === "string") arr.push(item);
							else if (item instanceof Y.Map) {
								var obj = {};
								item.forEach(function(val, key) {
									if (val instanceof Y.Text) obj[key] = val.toString();
									else obj[key] = val;
								});
								arr.push(obj);
							} else {
								arr.push(item);
							}
						});
						b[k] = arr;
					} else if (v instanceof Y.Map) {
						var obj = {};
						v.forEach(function(val, key) {
							if (val instanceof Y.Text) obj[key] = val.toString();
							else obj[key] = val;
						});
						b[k] = obj;
					} else {
						b[k] = v;
					}
				});
				result[blockId] = b;
			});
			return JSON.stringify(result);
		})()
	`)
	if err != nil {
		return nil, fmt.Errorf("readBlocks: %w", err)
	}

	result := make(map[string]map[string]any)
	if err := json.Unmarshal([]byte(val.String()), &result); err != nil {
		return nil, fmt.Errorf("parse blocks JSON: %w", err)
	}
	return result, nil
}

// ReadMeta reads the "meta" Y.Map from a document.
func (e *Engine) ReadMeta(docID int) (map[string]any, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.vm.Set("_docId", docID)
	val, err := e.vm.RunString(`
		(function() {
			var doc = globalThis._docs[_docId];
			var meta = doc.getMap("meta");
			var result = {};
			meta.forEach(function(v, k) {
				if (v instanceof Y.Text) result[k] = v.toString();
				else if (v instanceof Y.Array) {
					var arr = [];
					v.forEach(function(item) { arr.push(item); });
					result[k] = arr;
				} else {
					result[k] = v;
				}
			});
			return JSON.stringify(result);
		})()
	`)
	if err != nil {
		return nil, fmt.Errorf("readMeta: %w", err)
	}
	result := make(map[string]any)
	if err := json.Unmarshal([]byte(val.String()), &result); err != nil {
		return nil, fmt.Errorf("parse meta JSON: %w", err)
	}
	return result, nil
}

// NewDoc creates a new empty Y.Doc and returns its handle ID.
func (e *Engine) NewDoc() (int, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	val, err := e.vm.RunString(`
		(function() {
			var doc = new Y.Doc();
			var id = globalThis._docs.length;
			globalThis._docs.push(doc);
			return id;
		})()
	`)
	if err != nil {
		return 0, fmt.Errorf("newDoc: %w", err)
	}
	return int(val.ToInteger()), nil
}

// EncodeStateAsUpdate encodes the full state as base64.
func (e *Engine) EncodeStateAsUpdate(docID int) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.vm.Set("_docId", docID)
	val, err := e.vm.RunString(`
		(function() {
			var doc = globalThis._docs[_docId];
			var update = Y.encodeStateAsUpdate(doc);
			var binary = '';
			for (var i = 0; i < update.length; i++) {
				binary += String.fromCharCode(update[i]);
			}
			return btoa(binary);
		})()
	`)
	if err != nil {
		return "", fmt.Errorf("encodeStateAsUpdate: %w", err)
	}
	return val.String(), nil
}

// SaveStateVector returns the state vector as base64.
func (e *Engine) SaveStateVector(docID int) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.vm.Set("_docId", docID)
	val, err := e.vm.RunString(`
		(function() {
			var doc = globalThis._docs[_docId];
			var sv = Y.encodeStateVector(doc);
			var binary = '';
			for (var i = 0; i < sv.length; i++) {
				binary += String.fromCharCode(sv[i]);
			}
			return btoa(binary);
		})()
	`)
	if err != nil {
		return "", fmt.Errorf("saveStateVector: %w", err)
	}
	return val.String(), nil
}

// EncodeDelta encodes the delta from a saved state vector as base64.
func (e *Engine) EncodeDelta(docID int, stateVectorB64 string) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.vm.Set("_docId", docID)
	e.vm.Set("_svB64", stateVectorB64)
	val, err := e.vm.RunString(`
		(function() {
			var doc = globalThis._docs[_docId];
			var svBin = atob(_svB64);
			var sv = new Uint8Array(svBin.length);
			for (var i = 0; i < svBin.length; i++) sv[i] = svBin.charCodeAt(i);
			var update = Y.encodeStateAsUpdate(doc, sv);
			var binary = '';
			for (var i = 0; i < update.length; i++) {
				binary += String.fromCharCode(update[i]);
			}
			return btoa(binary);
		})()
	`)
	if err != nil {
		return "", fmt.Errorf("encodeDelta: %w", err)
	}
	return val.String(), nil
}

// InsertFormattedText creates a Y.Text with inline markdown formatting
// converted to proper Y.Text attributes (bold, italic, code, etc.).
// Returns the text content for verification.
func (e *Engine) InsertFormattedText(docID int, blockID, key, markdown string) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.vm.Set("_docId", docID)
	e.vm.Set("_blockId", blockID)
	e.vm.Set("_key", key)
	val, err := e.vm.RunString(fmt.Sprintf(`
		(function() {
			var _markdown = %s;
			var doc = globalThis._docs[_docId];
			var blocks = doc.getMap("blocks");
			var block = blocks.get(_blockId);
			if (!block) return "error: block not found";

			var text = new Y.Text();
			var segments = parseInlineMarkdown(_markdown);
			var pos = 0;
			for (var i = 0; i < segments.length; i++) {
				var seg = segments[i];
				text.insert(pos, seg.text, seg.attrs || {});
				pos += seg.text.length;
			}
			block.set(_key, text);
			return text.toString();
		})()
	`, strconv.QuoteToASCII(markdown)))
	if err != nil {
		return "", fmt.Errorf("insertFormattedText: %w", err)
	}
	return val.String(), nil
}

// CreateFormattedBlock creates a new block with properly formatted rich text.
func (e *Engine) CreateFormattedBlock(docID int, blockID, flavour, blockType, markdownText string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.vm.Set("_docId", docID)
	e.vm.Set("_blockId", blockID)
	e.vm.Set("_flavour", flavour)
	e.vm.Set("_blockType", blockType)
	_, err := e.vm.RunString(fmt.Sprintf(`
		(function() {
			var _markdown = %s;
			var doc = globalThis._docs[_docId];
			var blocks = doc.getMap("blocks");
			var block = new Y.Map();
			block.set("sys:id", _blockId);
			block.set("sys:flavour", _flavour);
			block.set("sys:children", new Y.Array());
			if (_blockType) {
				block.set("sys:type", _blockType);
				block.set("prop:type", _blockType);
			}

			var text = new Y.Text();
			var segments = parseInlineMarkdown(_markdown);
			var pos = 0;
			for (var i = 0; i < segments.length; i++) {
				var seg = segments[i];
				text.insert(pos, seg.text, seg.attrs || {});
				pos += seg.text.length;
			}
			block.set("prop:text", text);
			blocks.set(_blockId, block);
			return "ok";
		})()
	`, strconv.QuoteToASCII(markdownText)))
	if err != nil {
		return fmt.Errorf("createFormattedBlock: %w", err)
	}
	return nil
}

// RunScript executes arbitrary JS with access to Y and the docs array.
func (e *Engine) RunScript(script string) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	val, err := e.vm.RunString(script)
	if err != nil {
		return "", err
	}
	return val.String(), nil
}

// FreeDoc removes a doc reference to allow GC.
func (e *Engine) FreeDoc(docID int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.vm.Set("_docId", docID)
	e.vm.RunString(`globalThis._docs[_docId] = null;`)
}
