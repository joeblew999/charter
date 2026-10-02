// Upstream: tinygo-org/tinygo#5798 (when fixed: delete this file and its import in index.mjs)
// Makes Go timers fire on Cloudflare. Import it before anything creates a Go runtime (index.mjs does).
//
// On Cloudflare (not under local workerd) the clock only moves on I/O, and after a setTimeout(d)
// it has moved by exactly d rounded DOWN to a whole millisecond: setTimeout(1999.7) advances it
// 1999 ms, setTimeout(0.4) by nothing, however often it is repeated (measured 2026-10-01). TinyGo's
// scheduler sleeps until its next timer with setTimeout(ns / 1e6), a fraction, so when under a
// millisecond is left it re-arms a timer that never moves the clock, and the Go timer never fires:
// time.Sleep, time.NewTimer and context deadlines then hang until some other I/O happens. With a
// millisecond clock near 1.8e18 ns, float rounding alone leaves such a remainder about half the time.
//
// So every sleep TinyGo asks for is rounded UP to a whole millisecond.
const TinyGo = globalThis.Go;
if (!TinyGo) throw new Error("tinygo-clock.mjs: import build/wasm_exec.js first");

globalThis.Go = class extends TinyGo {
	constructor() {
		super();
		const sleepTicks = this.importObject.gojs["runtime.sleepTicks"];
		if (!sleepTicks) throw new Error("tinygo-clock.mjs: wasm_exec.js has no runtime.sleepTicks to wrap");
		this.importObject.gojs["runtime.sleepTicks"] = nanoseconds => sleepTicks(Math.ceil(Number(nanoseconds) / 1e6) * 1e6);
	}
};
