// The browser half of intl: the server already wrote every Value with the same Intl (intl.mjs) in its best guess of
// the viewer's zone. This redoes only what the server couldn't know: the viewer's own time zone (unless they chose
// one, <html data-tz>) and clock (<html data-hc>), and relative times as time passes.
//
//   <time datetime="…" data-intl="DateTimeFormat" data-opts='{"dateStyle":"full"}' data-args="[1783179045123]">…</time>
//
// data-intl is the Intl API (or "since": the datetime relative to now; "zone": a zone's name; "browser-zone": this
// browser's zone), data-method its method (default format), data-opts and data-args its options and arguments as
// JSON. data-template, if set, is a translated message with the result's place marked by U+E000. The language is the
// nearest lang attribute; a browser without full data for it (trimmed ICU) keeps the server's text. A
// <select data-intl-zones> gets every zone this browser knows, each named in the page's language.
//
// Elements are redone on load and whenever they're added or changed (htmx swaps, WebSocket pushes, morphs: one
// MutationObserver). window.intl exposes intl and parseNumber, so forms can send plain values back.
import { intl, since, zoneName, fullySupports, parseNumber } from "./intl.mjs";

const SLOT = "";
const root = document.documentElement;

function browserZone() {
  try {
    return new Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC";
  } catch {
    return "UTC";
  }
}

// The browser tells the server its own zone, once and again when it changes (travel): from the next page on the
// server renders in it (intl.Site), so the text doesn't change after it arrives. A chosen zone needs no report.
// A zone id ("America/New_York") is all characters a cookie value may hold.
if (!root.dataset.tz && !document.cookie.split("; ").includes("browser_tz=" + browserZone())) {
  document.cookie = "browser_tz=" + browserZone() + "; path=/; max-age=31536000; samesite=lax" +
    (location.protocol === "https:" ? "; secure" : "");
}

// viewerZone: the zone the viewer chose, if this browser knows it; else the browser's own.
function viewerZone() {
  const chosen = root.dataset.tz;
  if (chosen) {
    try {
      new Intl.DateTimeFormat("en", { timeZone: chosen });
      return chosen;
    } catch {}
  }
  return browserZone();
}

const supported = new Map();
function supports(lang) {
  if (!supported.has(lang)) supported.set(lang, fullySupports(lang));
  return supported.get(lang);
}

function redo(el) {
  const api = el.dataset.intl;
  const lang = el.closest("[lang]")?.lang || root.lang || "en";
  let text;
  if (api === "browser-zone") {
    text = browserZone();
  } else {
    if (!supports(lang)) return; // keep the server's text
    const prefs = { timeZone: viewerZone(), hourCycle: root.dataset.hc || undefined };
    if (api === "since") {
      const at = Date.parse(el.getAttribute("datetime"));
      text = since(lang, at);
      const title = intl(lang, "DateTimeFormat", { dateStyle: "medium", timeStyle: "short" }, "format", [at], prefs);
      if (title && el.title !== title) el.title = title;
    } else if (api === "zone") {
      text = zoneName(lang, JSON.parse(el.dataset.args || "[]")[0]);
    } else {
      text = intl(lang, api, el.dataset.opts || "{}", el.dataset.method || "format", el.dataset.args || "[]", prefs);
    }
  }
  if (!text) return; // this browser can't: keep the server's text
  const tpl = el.dataset.template;
  if (tpl && tpl.includes(SLOT)) text = tpl.replace(SLOT, () => text);
  if (el.textContent !== text) el.textContent = text;
  if (!el.hasAttribute("data-intl-done")) el.setAttribute("data-intl-done", ""); // redone by this browser
}

function fillZones(select) {
  if (select.dataset.intlZonesDone || !Intl.supportedValuesOf) return;
  select.dataset.intlZonesDone = "1";
  const lang = select.closest("[lang]")?.lang || root.lang || "en";
  const have = new Set([...select.options].map((o) => o.value));
  const frag = document.createDocumentFragment();
  for (const id of Intl.supportedValuesOf("timeZone")) {
    if (!have.has(id)) frag.append(new Option(zoneName(lang, id), id));
  }
  select.append(frag);
}

function update(node = document) {
  if (node.nodeType !== 1 && node !== document) return;
  if (node.nodeType === 1 && node.matches("[data-intl]")) redo(node);
  for (const el of node.querySelectorAll("[data-intl]")) redo(el);
  if (node.nodeType === 1 && node.matches("select[data-intl-zones]")) fillZones(node);
  for (const el of node.querySelectorAll("select[data-intl-zones]")) fillZones(el);
}

// Our own text changes are childList mutations of a [data-intl] element that add only text nodes: skipped.
const observer = new MutationObserver((records) => {
  const todo = new Set();
  for (const r of records) {
    if (r.type === "attributes") todo.add(r.target);
    for (const n of r.addedNodes) if (n.nodeType === 1) todo.add(n);
  }
  for (const n of todo) if (n.isConnected) update(n);
});

update();
// An htmx morph rewrites a value's text in place (a characterData change the observer doesn't see) and drops
// data-intl-done: redo what each swap brought, in the viewer's zone again.
document.addEventListener("htmx:after:swap", (e) => update(e.target));
observer.observe(document.body, {
  childList: true,
  subtree: true,
  attributes: true,
  attributeFilter: ["data-intl", "data-method", "data-opts", "data-args", "datetime", "lang"],
});
setInterval(() => document.querySelectorAll('[data-intl="since"]').forEach(redo), 30000);

window.intl = { update, intl, parseNumber };
