// What connects gsxui's theme editor to the app it is shown in. Everything that chooses, resolves and names a
// theme is gsxui's own, beside this file as gsxui has it (web/theme.js, web/theme-state.js, web/theme-preview.js);
// this only says where the editor starts, keeps what a reader chose, and lets a page of the app be the editor's
// live preview. charter check --fix writes it here, as it writes gsxui's three: do not edit.
//
// A reader's choice is kept by the browser as gsxui's share code and as the theme's CSS (gsxui's themeCSS), which
// app.Head puts on every page before its first paint.
const KEPT = "gsxui-preset";
const KEPT_CSS = "gsxui-preset-css";

// A page of the app shown inside the editor is its preview: gsxui's preview script takes each theme the editor
// sends and puts it on the page. It looks for what gsxui's own preview page has (the names of the theme's
// variables, a part for each style, a part for each of its two tabs), which are given here from the editor's own
// catalogue, in the page's head: the page itself stays as it is, whatever style or tab is chosen.
async function preview() {
  let schema;
  try {
    schema = JSON.parse(parent.document.querySelector("[data-theme-schema]").textContent);
  } catch {
    return; // framed by something that is no theme editor of this app
  }
  const mark = (name, value) => {
    const meta = document.createElement("meta");
    meta.setAttribute(name, value);
    document.head.append(meta);
  };
  mark("data-theme-preview-tokens", schema.tokenNames.join(","));
  for (const style of schema.styles) mark("data-theme-preview-style", style);
  for (const tab of parent.document.querySelectorAll("[data-theme-page-tab]")) mark("data-theme-preview-page", tab.dataset.themePageTab);
  await import("./web/theme-preview.js");
}

// The editor's page: it starts at what the reader chose before, else at the app's own theme, and every change
// gsxui's editor commits (it writes each to the address, as ?preset=) is kept.
async function editor(element) {
  const { decodeShare, encodeShare, themeCSS } = await import("./web/theme-state.js");
  const schema = JSON.parse(element.textContent);
  const own = JSON.parse(document.querySelector("[data-charter-preset]").textContent);
  const ownCode = encodeShare(own, schema);
  // The editor itself checks what it is given; a code it wrote is read back as it is.
  const any = { color: () => true, radius: () => true, fontFamily: () => true };
  const keep = (code) => {
    let preset;
    try {
      preset = decodeShare(code, schema, any);
    } catch {
      return;
    }
    try {
      if (code === ownCode) {
        localStorage.removeItem(KEPT);
        localStorage.removeItem(KEPT_CSS);
      } else {
        localStorage.setItem(KEPT, code);
        localStorage.setItem(KEPT_CSS, themeCSS(preset, schema));
      }
    } catch {}
    document.dispatchEvent(new CustomEvent("charter:preset"));
    // What the page says of the choice: its code, where gsxui's own editor shows it in every style, and, for a
    // style the app is not built in, how the app is built in it.
    for (const at of document.querySelectorAll("[data-charter-code]")) at.textContent = code;
    for (const at of document.querySelectorAll("[data-charter-style]")) at.textContent = preset.style;
    for (const link of document.querySelectorAll("[data-charter-gsxui]")) link.href = "https://ui.gsxhq.dev/theme?preset=" + encodeURIComponent(code);
    for (const note of document.querySelectorAll("[data-charter-other-style]")) note.hidden = preset.style === own.style;
    for (const button of document.querySelectorAll("[data-charter-own]")) button.disabled = code === ownCode;
  };
  const at = new URL(location.href);
  if (!at.searchParams.has("preset")) {
    let kept = null;
    try {
      kept = localStorage.getItem(KEPT);
    } catch {}
    at.searchParams.set("preset", kept ?? ownCode);
    history.replaceState(null, "", at);
  }
  const replace = history.replaceState.bind(history);
  history.replaceState = (state, title, to) => {
    replace(state, title, to);
    const code = new URL(to ?? location.href, location.href).searchParams.get("preset");
    if (code) keep(code);
  };
  keep(new URL(location.href).searchParams.get("preset"));
  document.addEventListener("click", (event) => {
    if (!event.target.closest?.("[data-charter-own]")) return;
    try {
      localStorage.removeItem(KEPT);
      localStorage.removeItem(KEPT_CSS);
    } catch {}
    location.assign(location.pathname);
  });
  await import("./web/theme.js");
}

if (window.parent !== window) preview();
const schemaElement = document.querySelector("[data-theme-schema]");
if (schemaElement && window.parent === window) editor(schemaElement);
