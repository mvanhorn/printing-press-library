#!/usr/bin/env node
"use strict";

const fs = require("fs");
const path = require("path");
const vm = require("vm");
const { JSDOM } = require("jsdom");

function readStdin() {
  return new Promise((resolve, reject) => {
    const chunks = [];
    process.stdin.on("data", (chunk) => chunks.push(chunk));
    process.stdin.on("end", () => resolve(Buffer.concat(chunks).toString("utf8")));
    process.stdin.on("error", reject);
  });
}

class StorageStub {
  constructor() {
    this.values = new Map();
  }
  getItem(key) {
    return this.values.has(String(key)) ? this.values.get(String(key)) : null;
  }
  setItem(key, value) {
    this.values.set(String(key), String(value));
  }
  removeItem(key) {
    this.values.delete(String(key));
  }
  clear() {
    this.values.clear();
  }
  key(index) {
    return Array.from(this.values.keys())[index] || null;
  }
  get length() {
    return this.values.size;
  }
}

class XHRStub {
  open() {}
  setRequestHeader() {}
  getAllResponseHeaders() {
    return "";
  }
  getResponseHeader() {
    return null;
  }
  send() {
    this.readyState = 4;
    this.status = 200;
    this.responseText = "";
    this.response = "";
    if (typeof this.onreadystatechange === "function") this.onreadystatechange();
    if (typeof this.onload === "function") this.onload();
  }
  abort() {}
}

function installGlobals() {
  const dom = new JSDOM("<html><head><script></script></head><body><div id=\"app\"></div></body></html>", {
    url: "https://partitaiva24.cloud/",
    pretendToBeVisual: true,
  });
  const storage = new StorageStub();

  Object.defineProperty(globalThis, "window", { value: dom.window, configurable: true, writable: true });
  Object.defineProperty(globalThis, "document", { value: dom.window.document, configurable: true, writable: true });
  Object.defineProperty(globalThis, "navigator", { value: dom.window.navigator, configurable: true, writable: true });
  Object.defineProperty(globalThis, "location", { value: dom.window.location, configurable: true, writable: true });
  Object.defineProperty(globalThis, "self", { value: dom.window, configurable: true, writable: true });
  globalThis.HTMLElement = dom.window.HTMLElement;
  globalThis.Element = dom.window.Element;
  globalThis.Node = dom.window.Node;
  globalThis.Image = dom.window.Image;
  globalThis.XMLHttpRequest = XHRStub;
  globalThis.localStorage = storage;
  globalThis.sessionStorage = new StorageStub();
  globalThis.fetch = async () => ({
    ok: true,
    status: 200,
    text: async () => "",
    json: async () => ({}),
    arrayBuffer: async () => new ArrayBuffer(0),
  });
  const jqueryStub = function (selector) {
    let element = null;
    if (typeof selector === "string" && selector.trim().startsWith("<")) {
      const tag = selector.replace(/[<>/]/g, "").split(/\s+/)[0] || "div";
      element = dom.window.document.createElement(tag);
    } else if (selector && selector.nodeType) {
      element = selector;
    }
    return {
      0: element,
      length: element ? 1 : 0,
      append: () => jqueryStub(selector),
      attr: () => jqueryStub(selector),
      data: () => undefined,
      find: () => jqueryStub(),
      on: () => jqueryStub(selector),
      ready: (fn) => {
        if (typeof fn === "function") fn();
        return jqueryStub(selector);
      },
      text: () => "",
    };
  };
  jqueryStub.ajax = async () => ({});
  jqueryStub.each = (items, fn) => {
    if (!items || typeof fn !== "function") return items;
    Object.keys(items).forEach((key) => fn.call(items[key], key, items[key]));
    return items;
  };
  jqueryStub.map = (items, fn) => {
    if (!items || typeof fn !== "function") return [];
    return Object.keys(items).map((key) => fn(items[key], key));
  };
  jqueryStub.extend = Object.assign;
  jqueryStub.fn = {
    dataTable: {
      render: {},
      ext: { search: [], type: { order: {} } },
    },
  };
  globalThis.jQuery = jqueryStub;
  globalThis.$ = jqueryStub;
  globalThis.rest_nonce = "";
  globalThis.ajax_url = "";
  globalThis.wpApiSettings = { nonce: "" };

  Object.defineProperty(dom.window, "XMLHttpRequest", { value: XHRStub, configurable: true, writable: true });
  Object.defineProperty(dom.window, "localStorage", { value: storage, configurable: true, writable: true });
  Object.defineProperty(dom.window, "sessionStorage", { value: globalThis.sessionStorage, configurable: true, writable: true });
  Object.defineProperty(dom.window, "fetch", { value: globalThis.fetch, configurable: true, writable: true });
  Object.defineProperty(dom.window, "__p24_render_pdf", { value: undefined, configurable: true, writable: true });
  Object.defineProperty(dom.window, "jQuery", { value: jqueryStub, configurable: true, writable: true });
  Object.defineProperty(dom.window, "$", { value: jqueryStub, configurable: true, writable: true });
  Object.defineProperty(dom.window, "rest_nonce", { value: "", configurable: true, writable: true });
  Object.defineProperty(dom.window, "ajax_url", { value: "", configurable: true, writable: true });
  Object.defineProperty(dom.window, "wpApiSettings", { value: { nonce: "" }, configurable: true, writable: true });
  const canvasContext = {
    clearRect: () => {},
    createImageData: (width, height) => ({ width, height, data: new Uint8ClampedArray(width * height * 4) }),
    drawImage: () => {},
    fillRect: () => {},
    getImageData: (x, y, width, height) => ({ width, height, data: new Uint8ClampedArray(width * height * 4) }),
    putImageData: () => {},
    measureText: (text) => ({ width: String(text || "").length * 6 }),
  };
  Object.defineProperty(dom.window.HTMLCanvasElement.prototype, "getContext", {
    value: () => canvasContext,
    configurable: true,
    writable: true,
  });
  Object.defineProperty(dom.window.HTMLCanvasElement.prototype, "toDataURL", {
    value: () => "data:image/png;base64,",
    configurable: true,
    writable: true,
  });
}

function normalizeInput(raw) {
  const input = JSON.parse(raw || "{}");
  if (!input.invoice || typeof input.invoice !== "object") {
    throw new Error("stdin JSON must include an invoice object");
  }
  return input;
}

function buildOptions(input) {
  const invoice = input.invoice;
  const user = input.user || {};
  const invoiceSettings = input.invoiceSettings || user.invoice_defaults || {};
  const header = user.header || invoice.from || invoiceSettings.header || user.invoice_defaults?.header || null;
  const vm = buildVM(invoice, invoiceSettings);

  return {
    outputType: "arraybuffer",
    returnJsPDFDocObject: false,
    fileName: `Doc N ${invoice.number || ""} del ${invoice.date || ""}`,
    orientationLandscape: false,
    logo: null,
    invoice_header: header,
    vm,
    // Footer is caller-controlled. Pass an empty object (not null) so MN's
    // defensive defaults don't crash on `.text` access. Empty text means the
    // SPA-default "Generato da PARTITA IVA 24 - www.partitaiva24.it" line is
    // suppressed; pass any non-empty string to set your own.
    footer: { text: typeof input.footerText === "string" ? input.footerText : "" },
    pageLabel: "Pag. ",
    locale: invoice?.to?.country === "IT" ? "it" : "en",
  };
}

function money(value) {
  const num = Number(value || 0);
  return num.toLocaleString("it-IT", { minimumFractionDigits: 2, maximumFractionDigits: 2 });
}

function lineNet(product) {
  const qty = Number(product.qty || 0);
  const price = Number(product.price || 0);
  const gross = qty * price;
  const discount = Number(product.discount || 0);
  if (!discount) return Number(product.netprice ?? gross);
  return product.discounttype === "percent" ? gross * (1 - discount / 100) : gross - discount;
}

function buildVM(invoice, invoiceSettings) {
  const products = Array.isArray(invoice.products) ? invoice.products : [];
  const taxable = products.reduce((sum, product) => sum + lineNet(product), 0);
  const vat = products.reduce((sum, product) => sum + lineNet(product) * Number(product.taxrate || product.vat || 0) / 100, 0);
  const stamp = Number(invoice.stamp || 0);
  const total = taxable + vat + stamp;

  const vm = {
    invoice,
    invoiceSettings,
    pageSize: "A4",
    get taxable() {
      return taxable;
    },
    get total() {
      return total;
    },
    get vat() {
      return vat;
    },
    get nettopay() {
      return total - this.totWitholdings;
    },
    get totWitholdings() {
      return 0;
    },
    formattedToAddress(address) {
      const city = [address.city, address.prov ? `(${address.prov})` : ""].filter(Boolean).join(" ");
      return [address.cap, city].filter(Boolean).join(" - ");
    },
    formattedTaxRates() {
      const byRate = {};
      for (const product of products) {
        const rate = String(Number(product.taxrate || product.vat || 0));
        const net = lineNet(product);
        const tax = net * Number(rate) / 100;
        if (!byRate[rate]) byRate[rate] = { net: 0, tax: 0 };
        byRate[rate].net += net;
        byRate[rate].tax += tax;
      }
      const out = {};
      for (const [rate, row] of Object.entries(byRate)) {
        out[rate] = { netprice: money(row.net), vat: money(row.tax) };
      }
      return out;
    },
    exemptionReasons() {
      return products
        .filter((product) => Number(product.taxrate || product.vat || 0) === 0)
        .map((product) => {
          const vatInfo = (invoiceSettings.vats || []).find((vatRow) => Number(vatRow.vat_id) === Number(product.vat_id)) || {};
          return {
            netprice: lineNet(product),
            info: {
              e_code: product.e_code || vatInfo.e_code || "",
              label: vatInfo.label || vatInfo.short_label || "",
            },
          };
        });
    },
    formattedVat() {
      return money(vat);
    },
    formattedTotal() {
      return money(total);
    },
    formattedNetToPay() {
      return money(this.nettopay);
    },
    formattedTotalDiscount() {
      return money(0);
    },
    formatCassa() {
      return "";
    },
    formatRitenuta() {
      return "";
    },
  };
  return vm;
}

function toBuffer(result) {
  const value = result?.arrayBuffer || result;
  if (Buffer.isBuffer(value)) return value;
  if (value instanceof ArrayBuffer) return Buffer.from(value);
  if (ArrayBuffer.isView(value)) return Buffer.from(value.buffer, value.byteOffset, value.byteLength);
  throw new Error("renderer did not return arrayBuffer bytes");
}

async function main() {
  const input = normalizeInput(await readStdin());
  installGlobals();

  const bundlePath = path.join(__dirname, "assets", "bundle.patched.js");
  const code = fs.readFileSync(bundlePath, "utf8");
  try {
    vm.runInThisContext(code, { filename: bundlePath });
  } catch (err) {
    const frames = err && err.stack ? err.stack.split("\n").filter((line) => line.length < 500).slice(0, 8).join("\n") : String(err);
    console.error(`SPA boot warning: ${frames}`);
  }

  const render = globalThis.__p24_render_pdf || globalThis.window?.__p24_render_pdf;
  if (typeof render !== "function") {
    console.error("partitaiva24 PDF renderer MN was not exported; run `make pdfgen` to refresh and patch the bundle");
    process.exit(2);
  }

  const result = await render(buildOptions(input));
  process.stdout.write(toBuffer(result));
}

main().catch((err) => {
  console.error(err && err.stack ? err.stack : String(err));
  process.exit(1);
});
