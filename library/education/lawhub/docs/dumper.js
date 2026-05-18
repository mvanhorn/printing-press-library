// Prototype LawHub dumper. Run in a logged-in LawHub browser tab during discovery.
// It intentionally does not scrape LSAT question content. It looks for obvious
// app state and visible attempt/test links so we can understand the data shape.
(() => {
  const links = Array.from(document.querySelectorAll('a[href]')).map(a => ({
    text: (a.innerText || '').trim().slice(0, 120),
    href: a.href,
  })).filter(x => /test|prep|score|attempt|review|library/i.test(x.text + ' ' + x.href));
  const storage = {};
  for (const area of [localStorage, sessionStorage]) {
    for (let i = 0; i < area.length; i++) {
      const k = area.key(i);
      if (!k) continue;
      const v = area.getItem(k) || '';
      if (/test|prep|score|attempt|review|user|token|auth/i.test(k + ' ' + v.slice(0, 200))) {
        storage[k] = v.slice(0, 1000);
      }
    }
  }
  return JSON.stringify({ url: location.href, title: document.title, links, storage_keys: Object.keys(storage), storage }, null, 2);
})();
