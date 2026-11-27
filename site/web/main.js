// What every page loads. Vite bundles it (charter build) and serves it while developing (charter dev).
import "virtual:gsx-devpanel"; // gsx's dev panel: Cmd-D / Ctrl-D
// htmx's module sets window.htmx, which the extension after it registers with.
import "htmx.org";
import "htmx.org/dist/ext/hx-sse.js";
// A link to another page of the app is asked for when it is pressed, before the click is done (app.Body boosts it).
import "htmx.org/dist/ext/hx-preload.js";
// The browser's half of the languages: charter build writes it (vite.config.ts says where "charter/intl" is).
import "charter/intl";
// gsxui's behaviours and its CSS (Tailwind).
import "./gsxui/index.js";
import "./gsxui/index.css";
