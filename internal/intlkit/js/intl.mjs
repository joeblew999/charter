// Intl, the same module with the same V8 Intl on a Worker (Go calls it through intl-worker.mjs) and in the browser
// (intl-browser.mjs imports it). Pure functions, no DOM.
//
// intl(lang, api, opts, method, args, prefs) is new Intl[api](lang, opts)[method](...args): any Intl API and any of
// its methods. opts is a JSON object of the API's options, args a JSON array of the method's arguments (instants
// as epoch milliseconds). The result is what Intl returns: a string, or JSON for anything else (a number, an array,
// an object; an Intl.Locale is its tag). "" when Intl can't. api "Intl" calls Intl's own functions
// (supportedValuesOf, getCanonicalLocales).
//
// prefs are the viewer's { timeZone, hourCycle }: Intl.DateTimeFormat uses them unless opts sets its own
// ("viewerClock": false in opts keeps the language's own clock, to show what it looks like).
//
// The rest is what Intl doesn't do: message (ICU MessageFormat, which V8 has no Intl.MessageFormat for), parseNumber
// (Intl formats but doesn't parse), since (which unit "3 minutes ago" is in) and zoneName (a zone's label).

export function intl(lang, api, opts, method, args, prefs = {}) {
  try {
    const o = typeof opts === "string" ? JSON.parse(opts || "{}") : opts ?? {};
    const a = typeof args === "string" ? JSON.parse(args || "[]") : args ?? [];
    const m = method || "format";
    if (api === "Intl") return result(Intl[m](...a));
    return result(make(Intl[api], lang, api === "DateTimeFormat" ? withPrefs(o, prefs) : o)[m](...a));
  } catch {
    return "";
  }
}

function result(x) {
  if (typeof x === "string") return x;
  if (x === undefined) return "undefined"; // Intl's own answer (getTimeZones of a locale without a region)
  if (x instanceof Intl.Locale) return x.toString();
  // Iterables (Segmenter's segments) as arrays; a segment's input is the text itself, so it's left out.
  if (x !== null && typeof x === "object" && typeof x[Symbol.iterator] === "function") x = Array.from(x, (v) => (v !== null && typeof v === "object" && "input" in v ? (({ input, ...s }) => s)(v) : v));
  return JSON.stringify(x);
}

function withPrefs(opts, prefs) {
  const { viewerClock, ...o } = opts;
  if (!o.timeZone && prefs.timeZone && knownZone(prefs.timeZone)) o.timeZone = prefs.timeZone;
  if (prefs.hourCycle && viewerClock !== false && o.hour12 === undefined && !o.hourCycle && (o.hour || o.timeStyle)) {
    o.hourCycle = prefs.hourCycle;
  }
  return o;
}

// knownZone reports whether this Intl knows time zone id (a cookie can name one that doesn't exist).
const zones = new Map();
function knownZone(id) {
  if (!zones.has(id)) {
    try {
      make(Intl.DateTimeFormat, "en", { timeZone: id });
      zones.set(id, true);
    } catch {
      zones.set(id, false);
    }
  }
  return zones.get(id);
}

// make is new Ctor(lang, opts), kept: building an Intl formatter costs far more than using one (a German
// DateTimeFormat: ~62 µs to build, ~2.5 µs to format a date, in workerd), and a page formats many values with a few
// language-and-option pairs. Kept for the life of the module (on Workers an isolate spans requests), at most
// MAX_FORMATTERS, the least recently used dropped first, so options sent from a form can't grow it without end.
const MAX_FORMATTERS = 200;
const formatters = new Map();
function make(Ctor, lang, opts) {
  const key = Ctor.name + "\u0000" + lang + "\u0000" + JSON.stringify(opts ?? null);
  let f = formatters.get(key);
  if (f) {
    formatters.delete(key); // most recently used last
  } else {
    f = new Ctor(lang, opts);
    if (formatters.size >= MAX_FORMATTERS) formatters.delete(formatters.keys().next().value);
  }
  formatters.set(key, f);
  return f;
}

// since is the instant ms relative to now ("3 minutes ago", Intl.RelativeTimeFormat, numeric "auto"), in the largest
// unit that keeps the number readable.
export function since(lang, ms, opts = {}, now = Date.now()) {
  const s = (Number(ms) - now) / 1000, abs = Math.abs(s);
  const [n, unit] = abs < 45 ? [s, "second"] : abs < 2700 ? [s / 60, "minute"] : abs < 79200 ? [s / 3600, "hour"]
    : abs < 2246400 ? [s / 86400, "day"] : abs < 27648000 ? [s / 2592000, "month"] : [s / 31536000, "year"];
  try {
    return make(Intl.RelativeTimeFormat, lang, { numeric: "auto", ...opts }).format(Math.round(n), unit);
  } catch {
    return "";
  }
}

// zoneName: "Europe/Berlin · Central European Time (GMT+2)", in lang.
export function zoneName(lang, id, now = Date.now()) {
  const part = (style) =>
    make(Intl.DateTimeFormat, lang, { timeZone: id, timeZoneName: style }).formatToParts(now).find((p) => p.type === "timeZoneName")?.value;
  let label = id.replaceAll("_", " ");
  try {
    label += " · " + part("longGeneric") + " (" + part("shortOffset") + ")";
  } catch {}
  return label;
}

// message formats one message of the catalogs mise-intl gen writes (intl-messages.mjs) in lang: a string, or a list of
// parts, each a string or ["a", i] (a text argument), ["n", i, style] (a number; style "integer"), ["p" or "o", i,
// offset, cases] (plural, selectordinal: an exact "=N" case, else the category Intl.PluralRules picks, else
// "other"), ["s", i, cases] (select) or ["#"] (the plural's number less its offset). args are strings, numbers as
// plain decimals; a text argument that starts with OWN is already in lang (another message), so it isn't isolated.
export function message(lang, msg, args = []) {
  const own = args.map((a) => typeof a === "string" && a.startsWith(OWN));
  args = args.map((a, i) => (own[i] ? a.slice(OWN.length) : a));
  return typeof msg === "string" ? msg : messageParts(lang, msg ?? [], args, "", 0, own);
}

// OWN marks a text argument in the message's own language (message.go's Translate): a page's title in
// "{title} · go-htmx4" needs no isolation, and a search result shows no stray marks around it.
const OWN = "\ue001";

function messageParts(lang, parts, args, pound, offset, own) {
  let out = "";
  for (const p of parts) {
    if (typeof p === "string") {
      out += p;
      continue;
    }
    switch (p[0]) {
      case "a": // text of unknown direction: isolated (FSI … PDI), so it can't reorder the words around it
        out += !args[p[1]] ? "" : own[p[1]] ? args[p[1]] : "\u2068" + args[p[1]] + "\u2069";
        break;
      case "n":
        out += number(lang, args[p[1]] ?? "", p[2] === "integer" ? { maximumFractionDigits: 0 } : {});
        break;
      case "#":
        out += pound === "" ? "#" : number(lang, String(Number(pound) - offset), {});
        break;
      case "p":
      case "o": {
        const value = args[p[1]] ?? "0", cases = p[3];
        const chosen = cases["=" + Number(value)] ??
          cases[make(Intl.PluralRules, lang, { type: p[0] === "o" ? "ordinal" : "cardinal" }).select(Number(value) - p[2])] ??
          cases.other;
        if (chosen) out += messageParts(lang, chosen, args, value, p[2], own);
        break;
      }
      case "s": {
        const chosen = p[2][args[p[1]]] ?? p[2].other;
        if (chosen) out += messageParts(lang, chosen, args, pound, offset, own);
        break;
      }
    }
  }
  return out;
}

function number(lang, value, opts) {
  try {
    return make(Intl.NumberFormat, lang, opts).format(value);
  } catch {
    return value;
  }
}

// fullySupports: this Intl has data for lang itself, not a fallback to another language (trimmed ICU data in some
// browsers resolves e.g. hi to en). The browser keeps the server's text when it doesn't.
export function fullySupports(lang) {
  try {
    return Intl.DateTimeFormat.supportedLocalesOf([lang]).length > 0 &&
      make(Intl.NumberFormat, lang).resolvedOptions().locale.split("-")[0] === lang.split("-")[0];
  } catch {
    return false;
  }
}

// parseNumber reads a number written the language's way ("1.234,5" in German, "١٬٢٣٤٫٥" in Arabic) back to a plain
// decimal string ("1234.5"), using the separators and digits this Intl formats with. "" when it isn't a number.
export function parseNumber(lang, text) {
  let parts;
  try {
    parts = make(Intl.NumberFormat, lang).formatToParts(-12345.6);
  } catch {
    return "";
  }
  const sym = (type) => parts.find((p) => p.type === type)?.value ?? "";
  const group = sym("group"), decimal = sym("decimal") || ".";
  const digits = make(Intl.NumberFormat, lang, { useGrouping: false }).format(9876543210).split("").reverse(); // 0…9
  let s = text.trim().replace(/[‎‏؜  \s]/g, "");
  if (group) s = s.split(group.replace(/[  ]/g, "")).join("");
  let out = "";
  for (const ch of s) {
    const d = digits.indexOf(ch);
    if (d >= 0) out += d;
    else if (/[0-9]/.test(ch)) out += ch;
    else if (ch === decimal) out += ".";
    else if (ch === "-" || ch === "−") out += "-";
    else if (ch === "+") continue;
    else return "";
  }
  return /^-?(\d+(\.\d+)?|\.\d+)$/.test(out) ? out : "";
}

// calendar is what a month calendar writes in lang, from Intl.DateTimeFormat, Intl.NumberFormat and Intl.Locale: the
// fields of gsxui's CalendarLocale and the first day of the week, joined by U+001E (lists inside by U+001F) for Go.
//   months, months inside a date, short months (January first), weekdays, narrow weekdays (Sunday first), the caption pattern
//   ("{month} {year}"), the day label pattern ("{weekday}, {month} {day}, {year}"), the ten digits ("" for 0-9), and
//   the first day of the week (1 Monday … 7 Sunday).
export function calendar(lang) {
  // A month calendar is Gregorian: a locale whose default calendar is another (fa: Persian) still names Gregorian months.
  const fmt = (o) => make(Intl.DateTimeFormat, lang, { timeZone: "UTC", calendar: "gregory", ...o });
  const months = [], inDate = [], short = [], weekdays = [], narrow = [];
  // A month's name is its part of the pattern it goes into ("1" in Japanese "2026年1月", whose 月 is the pattern's), so
  // name and pattern always fit together: the caption's standalone name ("январь 2026"), and the day label's name
  // inside a date ("15 января 2026 г.").
  const part = (o, d, type) => fmt(o).formatToParts(d).find((p) => p.type === type)?.value ?? "";
  const dayLabel = { weekday: "long", month: "long", day: "numeric", year: "numeric" };
  for (let m = 0; m < 12; m++) {
    const d = Date.UTC(2026, m, 15);
    months.push(part({ month: "long", year: "numeric" }, d, "month"));
    inDate.push(part(dayLabel, d, "month"));
    short.push(fmt({ month: "short" }).format(d));
  }
  for (let w = 0; w < 7; w++) { // 2026-01-04 is a Sunday
    const d = Date.UTC(2026, 0, 4 + w);
    weekdays.push(fmt({ weekday: "long" }).format(d));
    narrow.push(fmt({ weekday: "narrow" }).format(d));
  }
  // A pattern is a date formatted to parts, each field replaced by its placeholder.
  const pattern = (o) => fmt(o).formatToParts(Date.UTC(2026, 0, 15))
    .map((p) => (["weekday", "month", "day", "year"].includes(p.type) ? "{" + p.type + "}" : p.value)).join("");
  let digits = make(Intl.NumberFormat, lang, { useGrouping: false }).format(1234567890);
  digits = digits.slice(9) + digits.slice(0, 9); // 0…9
  if (digits === "0123456789") digits = "";
  let first = 7;
  try {
    const loc = new Intl.Locale(lang).maximize();
    first = (loc.getWeekInfo?.() ?? loc.weekInfo)?.firstDay ?? 7;
  } catch {}
  return [months.join("\u001f"), inDate.join("\u001f"), short.join("\u001f"), weekdays.join("\u001f"), narrow.join("\u001f"),
    pattern({ month: "long", year: "numeric" }), pattern(dayLabel), digits, String(first)].join("\u001e");
}

// segments is text split by Intl.Segmenter in lang: granularity "grapheme" (what a reader sees as one character),
// "word" (the word-like segments only) or "sentence" (without blank ones), joined by U+001F for Go.
export function segments(lang, text, granularity) {
  let parts = [...make(Intl.Segmenter, lang, { granularity }).segment(text)];
  if (granularity === "word") parts = parts.filter((p) => p.isWordLike);
  if (granularity === "sentence") parts = parts.filter((p) => p.segment.trim() !== "");
  return parts.map((p) => p.segment).join("\u001f");
}

// order is the indexes of items (a JSON array of strings) in the order Intl.Collator sorts them in lang with opts,
// comma-separated for Go.
export function order(lang, items, opts) {
  const c = make(Intl.Collator, lang, JSON.parse(opts || "{}"));
  const a = JSON.parse(items);
  return a.map((_, i) => i).sort((x, y) => c.compare(a[x], a[y]) || x - y).join(",");
}
