#!/usr/bin/env node
// orgo-chrome-bridge.js — Self-contained CDP bridge for Orgo VMs
// Zero npm dependencies. Uses only Node.js built-ins.
// Deployed into VMs via `cat << 'EOF' > /tmp/orgo-chrome-bridge.js`

const http = require("http");
const https = require("https");
const net = require("net");
const crypto = require("crypto");
const { EventEmitter } = require("events");
const { execSync, spawn } = require("child_process");

// ============================================================================
// Minimal WebSocket client — zero dependencies, Node.js built-ins only.
// Implements just enough of the WebSocket protocol for CDP communication.
// ============================================================================

class MinimalWebSocket extends EventEmitter {
  constructor(url) {
    super();
    this.readyState = 0; // CONNECTING
    this._buffer = Buffer.alloc(0);
    this._url = new URL(url);
    const key = crypto.randomBytes(16).toString("base64");

    const socket = net.createConnection(
      { host: this._url.hostname, port: parseInt(this._url.port) || 80 },
      () => {
        const path = this._url.pathname + (this._url.search || "");
        socket.write(
          `GET ${path} HTTP/1.1\r\n` +
          `Host: ${this._url.host}\r\n` +
          `Upgrade: websocket\r\n` +
          `Connection: Upgrade\r\n` +
          `Sec-WebSocket-Key: ${key}\r\n` +
          `Sec-WebSocket-Version: 13\r\n\r\n`
        );
      }
    );

    this._socket = socket;
    let upgraded = false;

    socket.on("data", (chunk) => {
      if (!upgraded) {
        // Look for the end of HTTP headers
        this._buffer = Buffer.concat([this._buffer, chunk]);
        const headerEnd = this._buffer.indexOf("\r\n\r\n");
        if (headerEnd === -1) return;
        const headers = this._buffer.slice(0, headerEnd).toString();
        if (!headers.includes("101")) {
          this.emit("error", new Error("WebSocket upgrade failed: " + headers.split("\r\n")[0]));
          socket.destroy();
          return;
        }
        upgraded = true;
        this.readyState = 1; // OPEN
        this.emit("open");
        // Process any remaining data after headers
        const remaining = this._buffer.slice(headerEnd + 4);
        this._buffer = Buffer.alloc(0);
        if (remaining.length > 0) this._processFrames(remaining);
      } else {
        this._processFrames(chunk);
      }
    });

    socket.on("close", () => { this.readyState = 3; this.emit("close"); });
    socket.on("error", (err) => { this.emit("error", err); });
  }

  _processFrames(data) {
    this._buffer = Buffer.concat([this._buffer, data]);
    while (this._buffer.length >= 2) {
      const firstByte = this._buffer[0];
      const secondByte = this._buffer[1];
      const opcode = firstByte & 0x0f;
      const masked = (secondByte & 0x80) !== 0;
      let payloadLength = secondByte & 0x7f;
      let offset = 2;

      if (payloadLength === 126) {
        if (this._buffer.length < 4) return;
        payloadLength = this._buffer.readUInt16BE(2);
        offset = 4;
      } else if (payloadLength === 127) {
        if (this._buffer.length < 10) return;
        payloadLength = Number(this._buffer.readBigUInt64BE(2));
        offset = 10;
      }

      if (masked) offset += 4;
      if (this._buffer.length < offset + payloadLength) return;

      let payload = this._buffer.slice(offset, offset + payloadLength);
      if (masked) {
        const mask = this._buffer.slice(offset - 4, offset);
        for (let i = 0; i < payload.length; i++) payload[i] ^= mask[i % 4];
      }

      this._buffer = this._buffer.slice(offset + payloadLength);

      if (opcode === 0x01) { // Text frame
        this.emit("message", payload.toString("utf-8"));
      } else if (opcode === 0x08) { // Close
        this.close();
      } else if (opcode === 0x09) { // Ping
        this._sendFrame(0x0a, payload); // Pong
      }
    }
  }

  _sendFrame(opcode, data) {
    const payload = Buffer.isBuffer(data) ? data : Buffer.from(data, "utf-8");
    const mask = crypto.randomBytes(4);
    const masked = Buffer.alloc(payload.length);
    for (let i = 0; i < payload.length; i++) masked[i] = payload[i] ^ mask[i % 4];

    let header;
    if (payload.length < 126) {
      header = Buffer.alloc(6);
      header[0] = 0x80 | opcode; // FIN + opcode
      header[1] = 0x80 | payload.length; // MASK + length
      mask.copy(header, 2);
    } else if (payload.length < 65536) {
      header = Buffer.alloc(8);
      header[0] = 0x80 | opcode;
      header[1] = 0x80 | 126;
      header.writeUInt16BE(payload.length, 2);
      mask.copy(header, 4);
    } else {
      header = Buffer.alloc(14);
      header[0] = 0x80 | opcode;
      header[1] = 0x80 | 127;
      header.writeBigUInt64BE(BigInt(payload.length), 2);
      mask.copy(header, 10);
    }

    this._socket.write(Buffer.concat([header, masked]));
  }

  send(data) {
    if (this.readyState !== 1) throw new Error("WebSocket not open");
    this._sendFrame(0x01, data);
  }

  close() {
    if (this.readyState < 2) {
      this.readyState = 2;
      try { this._sendFrame(0x08, Buffer.alloc(0)); } catch {}
      this._socket.end();
    }
  }
}

const WebSocket = MinimalWebSocket;

const PORT = 7331;
const CDP_PORT = 9222;
const MAX_CONSOLE_BUFFER = 500;
const MAX_NETWORK_BUFFER = 500;

// ============================================================================
// State
// ============================================================================

let cdpWs = null;
let cdpRequestId = 1;
const cdpPending = new Map(); // id -> { resolve, reject, timer }
const consoleBuffer = [];
const networkBuffer = [];
let currentTargetId = null;

// ============================================================================
// Accessibility Tree Script (ported from Claude-in-Chrome extension)
// ============================================================================

const ACCESSIBILITY_TREE_SCRIPT = `
(function() {
  window.__orgoElementMap = window.__orgoElementMap || {};
  window.__orgoRefCounter = window.__orgoRefCounter || 0;

  function getRole(el) {
    var role = el.getAttribute("role");
    if (role) return role;
    var tag = el.tagName.toLowerCase();
    var type = el.getAttribute("type");
    var roleMap = {
      a: "link", button: "button",
      input: type === "submit" || type === "button" ? "button"
        : type === "checkbox" ? "checkbox"
        : type === "radio" ? "radio"
        : type === "file" ? "button" : "textbox",
      select: "combobox", textarea: "textbox",
      h1: "heading", h2: "heading", h3: "heading",
      h4: "heading", h5: "heading", h6: "heading",
      img: "image", nav: "navigation", main: "main",
      header: "banner", footer: "contentinfo",
      section: "region", article: "article",
      aside: "complementary", form: "form",
      table: "table", ul: "list", ol: "list", li: "listitem", label: "label"
    };
    return roleMap[tag] || "generic";
  }

  function getName(el) {
    var tag = el.tagName.toLowerCase();
    if (tag === "select") {
      var opt = el.querySelector("option[selected]") || el.options[el.selectedIndex];
      if (opt && opt.textContent) return opt.textContent.trim();
    }
    var aria = el.getAttribute("aria-label");
    if (aria && aria.trim()) return aria.trim();
    var ph = el.getAttribute("placeholder");
    if (ph && ph.trim()) return ph.trim();
    var title = el.getAttribute("title");
    if (title && title.trim()) return title.trim();
    var alt = el.getAttribute("alt");
    if (alt && alt.trim()) return alt.trim();
    if (el.id) {
      var lbl = document.querySelector('label[for="' + el.id + '"]');
      if (lbl && lbl.textContent) return lbl.textContent.trim();
    }
    if (tag === "input") {
      var itype = el.getAttribute("type") || "";
      var val = el.getAttribute("value");
      if (itype === "submit" && val) return val.trim();
      if (el.value && el.value.length < 50) return el.value.trim();
    }
    if (["button", "a", "summary"].includes(tag)) {
      var txt = "";
      for (var i = 0; i < el.childNodes.length; i++) {
        if (el.childNodes[i].nodeType === Node.TEXT_NODE) txt += el.childNodes[i].textContent;
      }
      if (txt.trim()) return txt.trim();
    }
    if (tag.match(/^h[1-6]$/)) {
      var ht = el.textContent;
      if (ht) return ht.trim().substring(0, 100);
    }
    var directText = "";
    for (var j = 0; j < el.childNodes.length; j++) {
      if (el.childNodes[j].nodeType === Node.TEXT_NODE) directText += el.childNodes[j].textContent;
    }
    if (directText.trim().length >= 3) {
      var dt = directText.trim();
      return dt.length > 100 ? dt.substring(0, 100) + "..." : dt;
    }
    return "";
  }

  function isVisible(el) {
    var s = window.getComputedStyle(el);
    return s.display !== "none" && s.visibility !== "hidden" && s.opacity !== "0"
      && el.offsetWidth > 0 && el.offsetHeight > 0;
  }

  function isInteractive(el) {
    var tag = el.tagName.toLowerCase();
    return ["a", "button", "input", "select", "textarea", "details", "summary"].includes(tag)
      || el.getAttribute("onclick") !== null
      || el.getAttribute("tabindex") !== null
      || el.getAttribute("role") === "button"
      || el.getAttribute("role") === "link"
      || el.getAttribute("contenteditable") === "true";
  }

  function isStructural(el) {
    var tag = el.tagName.toLowerCase();
    return ["h1","h2","h3","h4","h5","h6","nav","main","header","footer","section","article","aside"].includes(tag)
      || el.getAttribute("role") !== null;
  }

  function shouldInclude(el, filter) {
    var tag = el.tagName.toLowerCase();
    if (["script","style","meta","link","title","noscript"].includes(tag)) return false;
    if (filter !== "all" && el.getAttribute("aria-hidden") === "true") return false;
    if (filter !== "all" && !isVisible(el)) return false;
    if (filter !== "all") {
      var rect = el.getBoundingClientRect();
      if (!(rect.top < window.innerHeight && rect.bottom > 0 && rect.left < window.innerWidth && rect.right > 0)) return false;
    }
    if (filter === "interactive") return isInteractive(el);
    if (isInteractive(el)) return true;
    if (isStructural(el)) return true;
    if (getName(el).length > 0) return true;
    var r = getRole(el);
    return r !== null && r !== "generic" && r !== "image";
  }

  function buildTree(el, depth, maxDepth, filter) {
    if (depth > maxDepth || !el || !el.tagName) return;
    var include = shouldInclude(el, filter);
    if (include) {
      var role = getRole(el);
      var name = getName(el);
      var ref = null;
      for (var k in window.__orgoElementMap) {
        if (window.__orgoElementMap[k] === el) { ref = k; break; }
      }
      if (!ref) {
        ref = "ref_" + (++window.__orgoRefCounter);
        window.__orgoElementMap[ref] = el;
      }
      var line = " ".repeat(depth) + role;
      if (name) line += ' "' + name.replace(/\\s+/g, " ").substring(0, 100).replace(/"/g, '\\\\"') + '"';
      line += " [" + ref + "]";
      if (el.getAttribute("href")) line += ' href="' + el.getAttribute("href") + '"';
      if (el.getAttribute("type")) line += ' type="' + el.getAttribute("type") + '"';
      lines.push(line);
    }
    if (el.children && depth < maxDepth) {
      for (var c = 0; c < el.children.length; c++) {
        buildTree(el.children[c], include ? depth + 1 : depth, maxDepth, filter);
      }
    }
  }

  var lines = [];
  var filterArg = arguments[0] || "all";
  var maxDepthArg = arguments[1] || 15;
  var maxCharsArg = arguments[2] || 50000;
  if (document.body) buildTree(document.body, 0, maxDepthArg, filterArg);

  // Cleanup dead refs
  for (var ref in window.__orgoElementMap) {
    if (!document.contains(window.__orgoElementMap[ref])) delete window.__orgoElementMap[ref];
  }

  var result = lines.join("\\n");
  if (maxCharsArg && result.length > maxCharsArg) {
    return JSON.stringify({
      error: "Output exceeds " + maxCharsArg + " chars (" + result.length + "). Try smaller depth or filter: interactive.",
      pageContent: "", viewport: { width: window.innerWidth, height: window.innerHeight }
    });
  }
  return JSON.stringify({
    pageContent: result,
    viewport: { width: window.innerWidth, height: window.innerHeight }
  });
})()
`;

// ============================================================================
// CDP Communication
// ============================================================================

function cdpSend(method, params = {}) {
  return new Promise((resolve, reject) => {
    if (!cdpWs || cdpWs.readyState !== 1) {
      return reject(new Error("CDP not connected"));
    }
    const id = cdpRequestId++;
    const timer = setTimeout(() => {
      cdpPending.delete(id);
      reject(new Error(`CDP timeout: ${method}`));
    }, 30000);
    cdpPending.set(id, { resolve, reject, timer });
    cdpWs.send(JSON.stringify({ id, method, params }));
  });
}

function handleCdpMessage(data) {
  let msg;
  try { msg = JSON.parse(data.toString()); } catch { return; }

  // Response to a request
  if (msg.id && cdpPending.has(msg.id)) {
    const { resolve, reject, timer } = cdpPending.get(msg.id);
    clearTimeout(timer);
    cdpPending.delete(msg.id);
    if (msg.error) reject(new Error(msg.error.message));
    else resolve(msg.result);
    return;
  }

  // Event
  if (msg.method === "Runtime.consoleAPICalled") {
    const args = (msg.params.args || []).map(a => a.value || a.description || "").join(" ");
    consoleBuffer.push({ type: msg.params.type, text: args, timestamp: Date.now() });
    if (consoleBuffer.length > MAX_CONSOLE_BUFFER) consoleBuffer.shift();
  }

  if (msg.method === "Network.requestWillBeSent") {
    const req = msg.params.request;
    networkBuffer.push({
      id: msg.params.requestId,
      method: req.method,
      url: req.url,
      timestamp: msg.params.timestamp,
      type: msg.params.type
    });
    if (networkBuffer.length > MAX_NETWORK_BUFFER) networkBuffer.shift();
  }

  if (msg.method === "Network.responseReceived") {
    const entry = networkBuffer.find(r => r.id === msg.params.requestId);
    if (entry) {
      entry.status = msg.params.response.status;
      entry.statusText = msg.params.response.statusText;
      entry.mimeType = msg.params.response.mimeType;
    }
  }

  // Clear stale element refs on page navigation
  if (msg.method === "Page.frameNavigated" && msg.params.frame && !msg.params.frame.parentId) {
    // Top-level frame navigated — clear all refs
    cdpSend("Runtime.evaluate", {
      expression: "window.__orgoElementMap = {}; window.__orgoRefCounter = 0; 'refs_cleared'",
      returnByValue: true
    }).catch(() => { /* page may not be ready yet */ });
  }
}

// ============================================================================
// Chrome + CDP Setup
// ============================================================================

async function getDebugTargets() {
  return new Promise((resolve, reject) => {
    http.get(`http://localhost:${CDP_PORT}/json/list`, (res) => {
      let body = "";
      res.on("data", (chunk) => body += chunk);
      res.on("end", () => {
        try { resolve(JSON.parse(body)); } catch (e) { reject(e); }
      });
    }).on("error", reject);
  });
}

async function launchChrome() {
  try {
    await getDebugTargets();
    console.log("[bridge] Chrome already running with CDP");
    return;
  } catch {
    // Chrome not running, launch it
  }

  console.log("[bridge] Launching Chrome...");
  // Detect DISPLAY from environment or common Orgo VM defaults
  const display = process.env.DISPLAY || ":99";
  const chrome = spawn("google-chrome", [
    "--no-sandbox", "--disable-gpu", "--disable-dev-shm-usage",
    "--remote-debugging-port=" + CDP_PORT,
    "--user-data-dir=/tmp/chrome-cdp-profile",
    "--window-size=1280,720", "--no-first-run",
    "--disable-default-apps", "--disable-extensions",
    "--disable-background-networking", "--disable-sync",
    "--disable-translate", "--metrics-recording-only",
    "--safebrowsing-disable-auto-update",
    "about:blank"
  ], { stdio: "ignore", detached: true, env: { ...process.env, DISPLAY: display } });
  chrome.unref();

  // Wait for CDP to become available
  for (let i = 0; i < 20; i++) {
    await new Promise(r => setTimeout(r, 500));
    try {
      await getDebugTargets();
      console.log("[bridge] Chrome CDP ready");
      return;
    } catch { /* retry */ }
  }
  throw new Error("Chrome failed to start with CDP");
}

async function connectCdp() {
  const targets = await getDebugTargets();
  const page = targets.find(t => t.type === "page") || targets[0];
  if (!page) throw new Error("No CDP targets available");

  currentTargetId = page.id;
  const wsUrl = page.webSocketDebuggerUrl;
  console.log("[bridge] Connecting to CDP:", wsUrl);

  return new Promise((resolve, reject) => {
    cdpWs = new WebSocket(wsUrl);
    cdpWs.on("open", async () => {
      console.log("[bridge] CDP connected");
      // Enable domains
      await cdpSend("Runtime.enable");
      await cdpSend("Network.enable");
      await cdpSend("Page.enable");
      resolve();
    });
    cdpWs.on("message", handleCdpMessage);
    cdpWs.on("close", () => { console.log("[bridge] CDP disconnected"); cdpWs = null; });
    cdpWs.on("error", (err) => { console.error("[bridge] CDP error:", err.message); reject(err); });
  });
}

// ============================================================================
// Ref Resolution — convert ref_N to {x, y} coordinates
// ============================================================================

async function resolveRef(ref) {
  const result = await cdpSend("Runtime.evaluate", {
    expression: `(function() {
      var el = window.__orgoElementMap && window.__orgoElementMap["${ref}"];
      if (!el || !document.contains(el)) return JSON.stringify({error: "Element not found"});
      var rect = el.getBoundingClientRect();
      return JSON.stringify({x: Math.round(rect.x + rect.width/2), y: Math.round(rect.y + rect.height/2), tag: el.tagName});
    })()`,
    returnByValue: true
  });
  return JSON.parse(result.result.value);
}

// ============================================================================
// HTTP API Handlers
// ============================================================================

const handlers = {
  "GET /health": async () => {
    const connected = cdpWs && cdpWs.readyState === 1;
    return { status: "ok", chrome: connected, pid: process.pid };
  },

  "POST /screenshot": async (body) => {
    const result = await cdpSend("Page.captureScreenshot", {
      format: body.format || "png",
      quality: body.quality || 80,
      ...(body.clip && { clip: body.clip })
    });
    return { image: result.data };
  },

  "POST /navigate": async (body) => {
    if (body.url === "back") {
      await cdpSend("Page.navigateToHistoryEntry", { entryId: -1 }).catch(() => {});
      const result = await cdpSend("Runtime.evaluate", { expression: "history.back(); document.title", returnByValue: true });
      return { action: "back" };
    }
    if (body.url === "forward") {
      await cdpSend("Runtime.evaluate", { expression: "history.forward()", returnByValue: true });
      return { action: "forward" };
    }
    let url = body.url;
    if (!url.startsWith("http")) url = "https://" + url;
    const nav = await cdpSend("Page.navigate", { url });
    // Wait for load
    await cdpSend("Page.lifecycleEvent").catch(() => {});
    await new Promise(r => setTimeout(r, 1000));
    const title = await cdpSend("Runtime.evaluate", { expression: "document.title", returnByValue: true });
    return { url, title: title.result.value, frameId: nav.frameId };
  },

  "POST /evaluate": async (body) => {
    const result = await cdpSend("Runtime.evaluate", {
      expression: body.expression,
      returnByValue: true,
      awaitPromise: body.awaitPromise || false
    });
    if (result.exceptionDetails) {
      return { error: result.exceptionDetails.text || "Evaluation error" };
    }
    return { result: result.result.value };
  },

  "POST /click": async (body) => {
    let x = body.x, y = body.y;
    if (body.ref) {
      const pos = await resolveRef(body.ref);
      if (pos.error) return pos;
      x = pos.x; y = pos.y;
    }
    const button = body.button || "left";
    const clickCount = body.double ? 2 : 1;
    await cdpSend("Input.dispatchMouseEvent", { type: "mousePressed", x, y, button, clickCount });
    await cdpSend("Input.dispatchMouseEvent", { type: "mouseReleased", x, y, button, clickCount });
    return { success: true, x, y };
  },

  "POST /type": async (body) => {
    if (body.text) {
      await cdpSend("Input.insertText", { text: body.text });
      return { success: true, typed: body.text };
    }
    return { error: "No text provided" };
  },

  "POST /key": async (body) => {
    // Handle key combos like "ctrl+a", "Enter", "Backspace"
    const keys = (body.key || "").split("+");
    const modifiers = [];
    let mainKey = keys[keys.length - 1];

    for (let i = 0; i < keys.length - 1; i++) {
      const mod = keys[i].toLowerCase();
      if (mod === "ctrl" || mod === "control") modifiers.push("ctrl");
      if (mod === "alt") modifiers.push("alt");
      if (mod === "shift") modifiers.push("shift");
      if (mod === "meta" || mod === "cmd" || mod === "command") modifiers.push("meta");
    }

    const modBits = modifiers.reduce((acc, m) => {
      if (m === "alt") return acc | 1;
      if (m === "ctrl") return acc | 2;
      if (m === "meta") return acc | 4;
      if (m === "shift") return acc | 8;
      return acc;
    }, 0);

    // Special key mapping
    const specialKeys = {
      enter: { key: "Enter", code: "Enter", keyCode: 13 },
      tab: { key: "Tab", code: "Tab", keyCode: 9 },
      backspace: { key: "Backspace", code: "Backspace", keyCode: 8 },
      delete: { key: "Delete", code: "Delete", keyCode: 46 },
      escape: { key: "Escape", code: "Escape", keyCode: 27 },
      arrowup: { key: "ArrowUp", code: "ArrowUp", keyCode: 38 },
      arrowdown: { key: "ArrowDown", code: "ArrowDown", keyCode: 40 },
      arrowleft: { key: "ArrowLeft", code: "ArrowLeft", keyCode: 37 },
      arrowright: { key: "ArrowRight", code: "ArrowRight", keyCode: 39 },
      space: { key: " ", code: "Space", keyCode: 32 },
      home: { key: "Home", code: "Home", keyCode: 36 },
      end: { key: "End", code: "End", keyCode: 35 },
    };

    const mapped = specialKeys[mainKey.toLowerCase()] || {
      key: mainKey.length === 1 ? mainKey : mainKey,
      code: mainKey.length === 1 ? "Key" + mainKey.toUpperCase() : mainKey,
      keyCode: mainKey.length === 1 ? mainKey.charCodeAt(0) : 0
    };

    await cdpSend("Input.dispatchKeyEvent", {
      type: "keyDown", modifiers: modBits,
      key: mapped.key, code: mapped.code,
      windowsVirtualKeyCode: mapped.keyCode, nativeVirtualKeyCode: mapped.keyCode
    });
    await cdpSend("Input.dispatchKeyEvent", {
      type: "keyUp", modifiers: modBits,
      key: mapped.key, code: mapped.code,
      windowsVirtualKeyCode: mapped.keyCode, nativeVirtualKeyCode: mapped.keyCode
    });

    return { success: true, key: body.key };
  },

  "POST /scroll": async (body) => {
    const x = body.x || 640;
    const y = body.y || 360;
    const deltaX = body.direction === "left" ? -120 : body.direction === "right" ? 120 : 0;
    const deltaY = body.direction === "up" ? -120 : body.direction === "down" ? 120 : 0;
    const amount = body.amount || 3;
    for (let i = 0; i < amount; i++) {
      await cdpSend("Input.dispatchMouseEvent", {
        type: "mouseWheel", x, y, deltaX, deltaY
      });
    }
    return { success: true, direction: body.direction, amount };
  },

  "POST /read_page": async (body) => {
    const filter = body.filter || "all";
    const depth = body.depth || 15;
    const maxChars = body.max_chars || 50000;
    const result = await cdpSend("Runtime.evaluate", {
      expression: `(${ACCESSIBILITY_TREE_SCRIPT.trim().slice(0, -2)})("${filter}", ${depth}, ${maxChars})`,
      returnByValue: true
    });
    return JSON.parse(result.result.value);
  },

  "POST /find": async (body) => {
    const query = (body.query || "").replace(/"/g, '\\"');
    const result = await cdpSend("Runtime.evaluate", {
      expression: `(function() {
        var query = "${query}".toLowerCase();
        var matches = [];
        var all = document.querySelectorAll("a, button, input, select, textarea, [role=button], [role=link], h1, h2, h3, h4, h5, h6, label, [aria-label], [placeholder]");
        for (var i = 0; i < all.length && matches.length < 20; i++) {
          var el = all[i];
          var text = (el.textContent || "").trim().substring(0, 200);
          var aria = el.getAttribute("aria-label") || "";
          var ph = el.getAttribute("placeholder") || "";
          var title = el.getAttribute("title") || "";
          var searchable = (text + " " + aria + " " + ph + " " + title).toLowerCase();
          if (searchable.includes(query)) {
            window.__orgoElementMap = window.__orgoElementMap || {};
            window.__orgoRefCounter = window.__orgoRefCounter || 0;
            var ref = null;
            for (var k in window.__orgoElementMap) {
              if (window.__orgoElementMap[k] === el) { ref = k; break; }
            }
            if (!ref) {
              ref = "ref_" + (++window.__orgoRefCounter);
              window.__orgoElementMap[ref] = el;
            }
            var rect = el.getBoundingClientRect();
            matches.push({
              ref: ref,
              tag: el.tagName.toLowerCase(),
              role: el.getAttribute("role") || el.tagName.toLowerCase(),
              text: text.substring(0, 100),
              ariaLabel: aria,
              placeholder: ph,
              rect: { x: Math.round(rect.x), y: Math.round(rect.y), w: Math.round(rect.width), h: Math.round(rect.height) }
            });
          }
        }
        return JSON.stringify(matches);
      })()`,
      returnByValue: true
    });
    return { elements: JSON.parse(result.result.value) };
  },

  "POST /form_input": async (body) => {
    if (!body.ref) return { error: "ref required" };
    const value = JSON.stringify(body.value);
    const result = await cdpSend("Runtime.evaluate", {
      expression: `(function() {
        var el = window.__orgoElementMap && window.__orgoElementMap["${body.ref}"];
        if (!el) return JSON.stringify({error: "Element not found"});
        var tag = el.tagName.toLowerCase();
        if (tag === "select") {
          el.value = ${value};
          el.dispatchEvent(new Event("change", {bubbles: true}));
        } else if (el.type === "checkbox" || el.type === "radio") {
          el.checked = ${value};
          el.dispatchEvent(new Event("change", {bubbles: true}));
        } else {
          el.value = ${value};
          el.dispatchEvent(new Event("input", {bubbles: true}));
          el.dispatchEvent(new Event("change", {bubbles: true}));
        }
        return JSON.stringify({success: true});
      })()`,
      returnByValue: true
    });
    return JSON.parse(result.result.value);
  },

  "POST /page_text": async () => {
    const result = await cdpSend("Runtime.evaluate", {
      expression: "document.body ? document.body.innerText : ''",
      returnByValue: true
    });
    return { text: result.result.value };
  },

  "POST /console": async (body) => {
    const pattern = body.pattern ? new RegExp(body.pattern, "i") : null;
    const limit = body.limit || 100;
    let msgs = consoleBuffer.slice();
    if (body.onlyErrors) msgs = msgs.filter(m => m.type === "error" || m.type === "exception");
    if (pattern) msgs = msgs.filter(m => pattern.test(m.text));
    msgs = msgs.slice(-limit);
    if (body.clear) consoleBuffer.length = 0;
    return { messages: msgs };
  },

  "POST /network": async (body) => {
    const limit = body.limit || 100;
    let reqs = networkBuffer.slice();
    if (body.urlPattern) reqs = reqs.filter(r => r.url.includes(body.urlPattern));
    reqs = reqs.slice(-limit);
    if (body.clear) networkBuffer.length = 0;
    return { requests: reqs };
  },

  "POST /tabs": async () => {
    const targets = await getDebugTargets();
    const tabs = targets
      .filter(t => t.type === "page")
      .map(t => ({ id: t.id, title: t.title, url: t.url }));
    return { tabs, currentTargetId };
  },

  "POST /new_tab": async (body) => {
    const url = body.url || "about:blank";
    const result = await cdpSend("Target.createTarget", { url });
    return { targetId: result.targetId };
  },

  "POST /switch_tab": async (body) => {
    if (!body.targetId) return { error: "targetId required" };
    await cdpSend("Target.activateTarget", { targetId: body.targetId });
    // Reconnect CDP to the new target
    const targets = await getDebugTargets();
    const target = targets.find(t => t.id === body.targetId);
    if (target && target.webSocketDebuggerUrl) {
      if (cdpWs) cdpWs.close();
      currentTargetId = body.targetId;
      return new Promise((resolve, reject) => {
        cdpWs = new WebSocket(target.webSocketDebuggerUrl);
        cdpWs.on("open", async () => {
          await cdpSend("Runtime.enable");
          await cdpSend("Network.enable");
          await cdpSend("Page.enable");
          resolve({ success: true, targetId: body.targetId });
        });
        cdpWs.on("message", handleCdpMessage);
        cdpWs.on("error", (e) => reject(e));
      });
    }
    return { success: true, targetId: body.targetId };
  },

  "POST /resize": async (body) => {
    const width = body.width || 1280;
    const height = body.height || 720;
    await cdpSend("Emulation.setDeviceMetricsOverride", {
      width, height, deviceScaleFactor: 1, mobile: false
    });
    return { success: true, width, height };
  },
};

// ============================================================================
// HTTP Server
// ============================================================================

function parseBody(req) {
  return new Promise((resolve) => {
    if (req.method === "GET") return resolve({});
    let body = "";
    req.on("data", (chunk) => body += chunk);
    req.on("end", () => {
      try { resolve(JSON.parse(body || "{}")); }
      catch { resolve({}); }
    });
  });
}

const server = http.createServer(async (req, res) => {
  const key = `${req.method} ${req.url.split("?")[0]}`;
  const handler = handlers[key];

  res.setHeader("Content-Type", "application/json");

  if (!handler) {
    res.writeHead(404);
    res.end(JSON.stringify({ error: `Unknown endpoint: ${key}` }));
    return;
  }

  try {
    const body = await parseBody(req);
    const result = await handler(body);
    res.writeHead(200);
    res.end(JSON.stringify(result));
  } catch (err) {
    res.writeHead(500);
    res.end(JSON.stringify({ error: err.message }));
  }
});

// ============================================================================
// Main
// ============================================================================

async function main() {
  console.log("[bridge] Starting Orgo Chrome Bridge...");

  await launchChrome();
  await connectCdp();

  server.listen(PORT, "127.0.0.1", () => {
    console.log(`[bridge] HTTP API listening on http://127.0.0.1:${PORT}`);
    console.log("[bridge] Ready.");
  });
}

main().catch((err) => {
  console.error("[bridge] Fatal:", err);
  process.exit(1);
});
