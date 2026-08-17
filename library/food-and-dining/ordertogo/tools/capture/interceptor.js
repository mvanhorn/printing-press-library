// OrderToGo order-request capture interceptor.
//
// Injected into a live ordertogo.com checkout page BEFORE placing a real order.
// Patches fetch + XMLHttpRequest to record the full POST /m/api/postmicmeshorder
// request (url, method, headers, body) and persist it to localStorage under
// 'otg_capture' so it SURVIVES the post-order navigation to /trackorder (a JS
// global would be wiped on that reload).
//
// Idempotent: re-injecting is a no-op. Does not alter the request.
//
// Read it back after the order with:
//   JSON.parse(localStorage.getItem('otg_capture'))
(function () {
  if (window.__otgHooked) return 'already-hooked';
  window.__otgHooked = true;

  var KEY = 'otg_capture';
  var MATCH = 'postmicmeshorder';

  function persist(rec) {
    try { localStorage.setItem(KEY, JSON.stringify(rec)); } catch (e) {}
  }

  // --- fetch ---
  var origFetch = window.fetch;
  window.fetch = function (input, init) {
    try {
      var url = (typeof input === 'string') ? input : (input && input.url) || '';
      if (url.indexOf(MATCH) >= 0) {
        var headers = {};
        var h = init && init.headers;
        if (h) {
          if (h.forEach) { h.forEach(function (v, k) { headers[k] = v; }); }
          else { Object.keys(h).forEach(function (k) { headers[k] = h[k]; }); }
        }
        persist({ via: 'fetch', url: url, method: (init && init.method) || 'GET', headers: headers, body: (init && init.body) || null, ts: Date.now() });
      }
    } catch (e) {}
    return origFetch.apply(this, arguments);
  };

  // --- XMLHttpRequest (axios) ---
  var oOpen = XMLHttpRequest.prototype.open;
  var oSet = XMLHttpRequest.prototype.setRequestHeader;
  var oSend = XMLHttpRequest.prototype.send;
  XMLHttpRequest.prototype.open = function (m, u) { this.__m = m; this.__u = u; this.__h = {}; return oOpen.apply(this, arguments); };
  XMLHttpRequest.prototype.setRequestHeader = function (k, v) { try { this.__h[k] = v; } catch (e) {} return oSet.apply(this, arguments); };
  XMLHttpRequest.prototype.send = function (b) {
    try {
      if (this.__u && this.__u.indexOf(MATCH) >= 0) {
        persist({ via: 'xhr', url: this.__u, method: this.__m, headers: this.__h, body: b, ts: Date.now() });
      }
    } catch (e) {}
    return oSend.apply(this, arguments);
  };

  return 'hooked';
})();
