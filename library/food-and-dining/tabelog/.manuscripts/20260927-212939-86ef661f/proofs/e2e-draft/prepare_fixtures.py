"""Produce small public-fact fixtures; never copy ephemeral tokens or review bodies."""
from __future__ import annotations

import hashlib
import html
from html.parser import HTMLParser
import json
from pathlib import Path
import re

RUN = Path(__file__).resolve().parents[2]
OUT = RUN / "working/tabelog-pp-cli/e2e/testdata"
OUT.mkdir(parents=True, exist_ok=True)
VOID = {"area", "base", "br", "col", "embed", "hr", "img", "input", "link", "meta", "param", "source", "track", "wbr"}
ATTRS = {"class", "id", "href", "rel", "title", "name", "value", "type", "checked", "selected", "aria-label", "role", "data-rst-id", "data-detail-url", "data-keyword-name", "data-keyword-type", "data-lst-are", "data-lst-prf", "data-pal", "data-station-id", "data-site-name"}

class Node:
    def __init__(self, tag, attrs=(), children=None):
        self.tag, self.attrs, self.children = tag, dict(attrs), children or []
    def classes(self):
        return set(self.attrs.get("class", "").split())

class Tree(HTMLParser):
    def __init__(self):
        super().__init__(convert_charrefs=True)
        self.root = Node("root")
        self.stack = [self.root]
    def handle_starttag(self, tag, attrs):
        node = Node(tag, attrs)
        self.stack[-1].children.append(node)
        if tag not in VOID:
            self.stack.append(node)
    def handle_startendtag(self, tag, attrs):
        self.handle_starttag(tag, attrs)
        if tag not in VOID:
            self.handle_endtag(tag)
    def handle_endtag(self, tag):
        for i in range(len(self.stack)-1, 0, -1):
            if self.stack[i].tag == tag:
                self.stack = self.stack[:i]
                break
    def handle_data(self, data):
        self.stack[-1].children.append(data)

def nodes(node):
    yield node
    for child in node.children:
        if isinstance(child, Node):
            yield from nodes(child)

def text(node):
    return " ".join("".join(text(x) if isinstance(x, Node) else x for x in node.children).split())

def render(node):
    if isinstance(node, str):
        return html.escape(re.sub(r"\s+", " ", node), quote=False)
    classes = node.classes()
    if node.tag in {"img", "style", "iframe", "noscript", "meta", "link"} or classes & {"list-rst__rst-photo", "list-rst__pr"}:
        return ""
    if node.attrs.get("id", "").startswith("js-booking"):
        return ""
    if re.search(r"csrf|nonce|token|auth|session|cookie", node.attrs.get("name", ""), re.I):
        return ""
    if node.tag == "script":
        if node.attrs.get("type") != "application/ld+json":
            return ""
        try:
            data = json.loads("".join(x for x in node.children if isinstance(x, str)))
        except (ValueError, TypeError):
            return ""
        if isinstance(data, dict) and data.get("@type") == "Restaurant":
            keep = {"@context", "@type", "@id", "name", "address", "geo", "priceRange", "servesCuisine", "telephone", "aggregateRating"}
            data = {k: v for k, v in data.items() if k in keep}
        elif not (isinstance(data, dict) and data.get("@type") == "BreadcrumbList"):
            return ""
        return '<script type="application/ld+json">' + json.dumps(data, ensure_ascii=False) + '</script>'
    attrs = []
    for key, value in node.attrs.items():
        if key not in ATTRS:
            continue
        if key == "href" and value and not (value.startswith(("https://tabelog.com/en/", "/en/", "#"))):
            continue
        attrs.append(" " + key if value is None else " " + key + '="' + html.escape(value, quote=True) + '"')
    inner = "".join(render(x) for x in node.children)
    if node.tag == "root":
        return inner
    return "<" + node.tag + "".join(attrs) + ">" + ("" if node.tag in VOID else inner + "</" + node.tag + ">")

def selected(node, wanted):
    if node.tag == "title" or node.classes() & wanted or (node.tag == "script" and node.attrs.get("type") == "application/ld+json"):
        return [node]
    out = []
    for child in node.children:
        if isinstance(child, Node):
            out.extend(selected(child, wanted))
    return out

sources = {
    "tokyo-ranked.html": ("discovery/public-source/tabelog.com-en-tokyo-rstLst-SrtT-rt-86c815e79fecbf3e.html", "listing"),
    "ginza-bars.html": ("discovery/native/ginza-bars-dinner5000.html", "listing"),
    "ginza-bars-page2.html": ("discovery/native-confirm/ginza-budget-page2.html", "listing"),
    "ginza-lunch.html": ("discovery/native/ginza-lunch1000to2000.html", "listing"),
    "ginza-keyword.html": ("discovery/native/ginza-keyword-sushi.html", "listing"),
    "station-shinjuku.html": ("discovery/native-confirm/station-shinjuku-ranked.html", "listing"),
    "sushi-detail.html": ("discovery/native-confirm/restaurant-detail.html", "detail"),
    "english-home.html": ("discovery/native-confirm/english-home.html", "home"),
    "suggest-ginza.json": ("discovery/native-ajax/suggest-ginza.json", "json"),
    "suggest-shinjuku.json": ("discovery/native-ajax/suggest-shinjuku.json", "json"),
    "suggest-bar.json": ("discovery/native-ajax/suggest-bar.json", "json"),
}
manifest = []
for name, (relative, kind) in sources.items():
    original = (RUN / relative).read_bytes()
    if kind == "json":
        result = json.dumps(json.loads(original), ensure_ascii=False, separators=(",", ":")) + "\n"
    else:
        tree = Tree()
        tree.feed(original.decode())
        if kind == "listing":
            roots = selected(tree.root, {"c-breadcrumb", "list-condition", "list-sidebar", "navi-rstlst", "c-page-count", "list-rst", "c-pagination", "rstlist-notfound"})
        elif kind == "detail":
            roots = selected(tree.root, {"c-breadcrumb", "rst-status-badge-red", "rdheader-title-data", "rdheader-info-data", "rstinfo-table", "c-alert"})
        else:
            # Only public location/category anchors; no homepage editorial/review body.
            roots = [n for n in nodes(tree.root) if n.tag == "title" or (
                n.tag == "a" and (
                    re.fullmatch(r"https://tabelog\.com/en/[a-z_-]+/", n.attrs.get("href", ""))
                    or "/rstLst/" in n.attrs.get("href", "")
                    or "data-pal" in n.attrs or "data-site-name" in n.attrs
                )
            )]
        result = '<!doctype html><html lang="en"><body>\n' + "\n".join(render(x) for x in roots) + '\n</body></html>\n'
    (OUT / name).write_text(result)
    manifest.append({"fixture": name, "source_path": relative, "original_sha256": hashlib.sha256(original).hexdigest(), "sanitized_sha256": hashlib.sha256(result.encode()).hexdigest(), "original_bytes": len(original), "sanitized_bytes": len(result.encode()), "transform": "Preserve public semantic source roots and canonical links; strip photos, reviewer bodies, non-JSONLD scripts, telemetry, ephemeral attributes, CSRF/nonce inputs. JSONLD keeps Restaurant/BreadcrumbList facts only." if kind != "json" else "Re-serialize the public typed suggestions array; preserve all source fields."})

# Narrow, explicitly synthetic missing-source-fact conditions for saved evidence tests.
tree = Tree()
tree.feed((OUT / "ginza-bars.html").read_text())
for card in nodes(tree.root):
    if card.attrs.get("data-rst-id") == "13120361":
        for element in nodes(card):
            if "list-rst__area-genre" in element.classes():
                element.children = ["Ginza Sta. 351m / -"]
    if card.attrs.get("data-rst-id") == "13224720":
        for element in nodes(card):
            if element.tag == "li" and any(n.attrs.get("aria-label") == "Average dinner price" for n in nodes(element)):
                for value in nodes(element):
                    if "c-rating-v3__val" in value.classes():
                        value.children = ["-"]
mutation = render(tree.root) + "\n"
(OUT / "ginza-bars-unknown.html").write_text(mutation)
manifest.append({"fixture": "ginza-bars-unknown.html", "parent_fixture": "ginza-bars.html", "sha256": hashlib.sha256(mutation.encode()).hexdigest(), "transform": "Only ID13120361 category label becomes '-' and ID13224720 average dinner price becomes '-'; retain canonical IDs/areas/other facts. These are synthetic source-unknown conditions, not live claims."})

(OUT / "fixture-manifest.json").write_text(json.dumps(manifest, indent=2) + "\n")
oracle = json.loads((RUN / "proofs/fixtures/oracle.json").read_text())
(OUT / "oracle.json").write_text(json.dumps(oracle, ensure_ascii=False, indent=2) + "\n")
print(json.dumps({"fixtures": len(manifest), "sanitized_bytes": sum(p.stat().st_size for p in OUT.glob("*.html")), "directory": str(OUT)}))
