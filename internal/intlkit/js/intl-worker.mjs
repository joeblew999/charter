// What Go calls (package intl): intl.mjs with the app's message catalogs (intl-messages.mjs, from mise-intl gen),
// every argument a string ("" for none), so Go in a Worker's wasm and go test (intltest, the same module on workerd)
// call it the same way. The Worker entry puts it on globalThis: globalThis.goIntl = goIntl.
import * as intl from "./intl.mjs";
import catalogs from "./intl-messages.mjs";

export const goIntl = {
  intl: (lang, api, opts, method, args, timeZone, hourCycle) => intl.intl(lang, api, opts, method, args, { timeZone, hourCycle }),
  since: (lang, ms) => intl.since(lang, ms),
  zoneName: (lang, id) => intl.zoneName(lang, id),
  parseNumber: (lang, text) => intl.parseNumber(lang, text),
  calendar: (lang) => intl.calendar(lang),
  segments: (lang, text, granularity) => intl.segments(lang, text, granularity),
  order: (lang, items, opts) => intl.order(lang, items, opts),
  // Message index of catalog (a locale id, or "en-XA": pseudo-localized) in lang.
  message: (lang, catalog, index, ...args) => intl.message(lang, catalogs[catalog]?.[Number(index)], args),
};
