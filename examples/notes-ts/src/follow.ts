// follow(): the one way a client receives a live, ordered, gap-free feed (docs/realtime.md, rule 3).
// Both transports (SSE `notes.watch`, WebSocket `/api/notes/live`) are thin adapters over it.
//
// The log (D1) is the source of truth and the item id is the only position. The live source (the
// hub Durable Object) is only a wake-up signal and may drop, restart, or deliver out of order:
// - subscribe first, then catch up from the log (id > after), then go live;
// - a live item is yielded directly only when it is the next id; otherwise the log is read, which
//   fills gaps and restores order (a single SQLite writer makes a visible id imply all lower ones);
// - on a hub error, resubscribe and catch up from the last id yielded;
// - when idle for `recheckMs`, catch up from the log anyway, so a hub that closes silently costs at
//   most that much delay, never a lost item.
// It throws only after `maxFailures` consecutive failed subscribes, so the adapter can end the
// stream explicitly (rule 5); an abort ends it quietly.

export interface FollowSource<T extends { id: number }> {
	/** Subscribe to live items; `onError` when the subscription is broken. Returns unsubscribe. */
	subscribe(listener: (item: T) => void, onError: (error: unknown) => void): Promise<() => Promise<void>>;
	/** Items with id > after, ascending, at most `limit`. */
	since(after: number, limit: number): Promise<T[]>;
	/** The newest id in the log (0 when empty): where a follower without `after` starts. */
	latest(): Promise<number>;
}

export interface FollowOptions {
	/** Resume position: yield items with id > after. Absent: only items newer than now. */
	after?: number | undefined;
	signal?: AbortSignal | undefined;
	/** Read the log when nothing has arrived for this long. */
	recheckMs?: number;
	/** Wait between resubscribes after a hub failure. */
	retryMs?: number;
	/** Consecutive failed subscribes before giving up (throws). */
	maxFailures?: number;
	/** Page size for log reads. */
	pageSize?: number;
	/** Called when the live subscription breaks (before resubscribing): for logs. */
	onBroken?: (error: unknown) => void;
}

export async function* follow<T extends { id: number }>(source: FollowSource<T>, options: FollowOptions = {}): AsyncGenerator<T> {
	const { signal, recheckMs = 30_000, retryMs = 1_000, maxFailures = 5, pageSize = 100, onBroken } = options;
	let last = options.after ?? (await source.latest());
	let failures = 0;

	// Wakes the loop on a live item, a hub error, an abort or a timeout.
	let wake: (() => void) | undefined;
	const nudge = () => wake?.();
	signal?.addEventListener("abort", nudge);
	const sleep = (ms: number) => new Promise<boolean>(resolve => {
		const timer = setTimeout(() => { wake = undefined; resolve(true); }, ms);
		wake = () => { clearTimeout(timer); wake = undefined; resolve(false); };
		if (signal?.aborted) wake();
	});

	// Everything the log has after `last`, in pages.
	async function* catchUp(): AsyncGenerator<T> {
		for (;;) {
			const page = await source.since(last, pageSize);
			for (const item of page) if (item.id > last) { last = item.id; yield item; }
			if (page.length < pageSize) return;
		}
	}

	try {
		while (!signal?.aborted) {
			const queue: T[] = [];
			let broken: unknown;
			let unsubscribe: (() => Promise<void>) | undefined;
			try {
				unsubscribe = await source.subscribe(item => { queue.push(item); nudge(); }, error => { broken = error ?? new Error("subscription broken"); nudge(); });
			} catch (error) {
				if (++failures >= maxFailures) throw error;
				await sleep(retryMs);
				continue;
			}
			try {
				yield* catchUp();
				failures = 0;
				while (!signal?.aborted && broken === undefined) {
					const item = queue.shift();
					if (item) {
						if (item.id === last + 1) { last = item.id; yield item; }
						else if (item.id > last) yield* catchUp(); // a gap or out of order: the log decides
						continue;
					}
					if (await sleep(recheckMs)) yield* catchUp(); // idle: covers a hub that closed silently
				}
			} finally {
				await unsubscribe?.().catch(() => {});
			}
			if (broken !== undefined && !signal?.aborted) { onBroken?.(broken); await sleep(retryMs); }
		}
	} finally {
		signal?.removeEventListener("abort", nudge);
	}
}
