import { describe, expect, it } from "vitest";
import { follow, type FollowSource } from "../src/follow.ts";

type Item = { id: number };

// A fake log plus hub: `add` appends to the log; `publish` delivers live (or not, to simulate loss);
// `breakHub` fails every current subscription; `failSubscribes` makes the next N subscribes throw.
function fakeSource(initial = 0) {
	const log: Item[] = Array.from({ length: initial }, (_, i) => ({ id: i + 1 }));
	let listeners: { listener: (i: Item) => void; onError: (e: unknown) => void }[] = [];
	let failNext = 0;
	const stats = { subscribes: 0, sinceCalls: 0 };
	const source: FollowSource<Item> = {
		async subscribe(listener, onError) {
			stats.subscribes++;
			if (failNext > 0) { failNext--; throw new Error("hub unavailable"); }
			const entry = { listener, onError };
			listeners.push(entry);
			return async () => { listeners = listeners.filter(l => l !== entry); };
		},
		async since(after, limit) { stats.sinceCalls++; return log.filter(i => i.id > after).slice(0, limit); },
		async latest() { return log.at(-1)?.id ?? 0; },
	};
	return {
		source, stats, log,
		add(live = true) { const item = { id: log.length + 1 }; log.push(item); if (live) for (const l of listeners) l.listener(item); return item; },
		publish(item: Item) { for (const l of listeners) l.listener(item); },
		breakHub() { const current = listeners; listeners = []; for (const l of current) l.onError(new Error("closed 1006")); },
		failSubscribes(n: number) { failNext = n; },
		get subscribers() { return listeners.length; },
	};
}

// Collect ids from a follower until `n` arrive (or timeout), driving `act` once it is attached.
async function take(gen: AsyncGenerator<Item>, n: number, timeoutMs = 2000) {
	const out: number[] = [];
	const timer = setTimeout(() => void gen.return(undefined), timeoutMs);
	for await (const item of gen) { out.push(item.id); if (out.length >= n) break; }
	clearTimeout(timer);
	return out;
}
const tick = (ms = 5) => new Promise(r => setTimeout(r, ms));
const fast = { recheckMs: 50, retryMs: 5 };

describe("follow()", () => {
	it("catches up from the log after `after`, then goes live, in order", async () => {
		const f = fakeSource(5);
		const got = take(follow(f.source, { after: 2, ...fast }), 5);
		await tick(); f.add(); f.add();
		expect(await got).toEqual([3, 4, 5, 6, 7]);
	});

	it("without `after` starts from now (no history)", async () => {
		const f = fakeSource(3);
		const got = take(follow(f.source, fast), 2);
		await tick(); f.add(); f.add();
		expect(await got).toEqual([4, 5]);
	});

	it("an item created between subscribe and catch-up is yielded once", async () => {
		const f = fakeSource(0);
		const origSince = f.source.since;
		let first = true;
		f.source.since = async (after, limit) => { if (first) { first = false; f.add(); } return origSince(after, limit); };
		const got = take(follow(f.source, { after: 0, ...fast }), 2);
		await tick(); f.add();
		expect(await got).toEqual([1, 2]);
	});

	it("a hub drop is invisible: it resubscribes and catches up what was missed", async () => {
		const f = fakeSource(0);
		const got = take(follow(f.source, { after: 0, ...fast }), 4);
		await tick(); f.add();
		f.breakHub();
		f.add(false); f.add(false); // created while no one is subscribed
		await tick(30); f.add();
		expect(await got).toEqual([1, 2, 3, 4]);
		expect(f.stats.subscribes).toBe(2);
	});

	it("a hub that closes silently costs at most recheckMs, not items", async () => {
		const f = fakeSource(0);
		const got = take(follow(f.source, { after: 0, ...fast }), 3);
		await tick(); f.add(false); f.add(false); f.add(false); // never delivered live
		expect(await got).toEqual([1, 2, 3]);
	});

	it("out-of-order live delivery is reordered from the log, no duplicates", async () => {
		const f = fakeSource(0);
		const got = take(follow(f.source, { after: 0, ...fast }), 3);
		await tick();
		const a = f.add(false), b = f.add(false), c = f.add(false);
		f.publish(c); f.publish(a); f.publish(b);
		expect(await got).toEqual([1, 2, 3]);
		void a; void b;
	});

	it("gives up after maxFailures consecutive failed subscribes (so the adapter can close)", async () => {
		const f = fakeSource(0);
		f.failSubscribes(10);
		await expect(take(follow(f.source, { after: 0, ...fast, maxFailures: 3 }), 1)).rejects.toThrow("hub unavailable");
		expect(f.stats.subscribes).toBe(3);
	});

	it("recovers when the hub comes back before maxFailures", async () => {
		const f = fakeSource(0);
		f.failSubscribes(2);
		const got = take(follow(f.source, { after: 0, ...fast, maxFailures: 5 }), 1);
		await tick(40); f.add();
		expect(await got).toEqual([1]);
	});

	it("abort ends the stream quietly and unsubscribes", async () => {
		const f = fakeSource(0);
		const ac = new AbortController();
		const gen = follow(f.source, { after: 0, signal: ac.signal, ...fast });
		const done = (async () => { const out: number[] = []; for await (const i of gen) out.push(i.id); return out; })();
		await tick(); f.add();
		await tick(); ac.abort();
		expect(await done).toEqual([1]);
		expect(f.subscribers).toBe(0);
	});

	it("pages through a long catch-up", async () => {
		const f = fakeSource(250);
		expect(await take(follow(f.source, { after: 0, ...fast, pageSize: 100 }), 250)).toHaveLength(250);
	});
});
