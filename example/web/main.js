// What every page loads. Vite bundles it (charter build) and serves it while developing (charter dev).
import "virtual:gsx-devpanel"; // gsx's dev panel: Cmd-D / Ctrl-D
// htmx's module sets window.htmx, which the extension after it registers with.
import "htmx.org";
import "htmx.org/dist/ext/hx-sse.js";
// The browser's half of the languages: charter build writes it (vite.config.ts says where "charter/intl" is).
import "charter/intl";
// gsxui's behaviours and its CSS (Tailwind).
import "./gsxui/index.js";
import "./gsxui/index.css";
