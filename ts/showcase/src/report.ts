/** What the harness's routes (/api/sdk-test, /api/ws-test) answer, and scripts/harness.ts reads: each
 * check by name, and whether all passed. */
export interface Report {
	passed: boolean;
	results: Record<string, { ok: boolean; detail: unknown }>;
}
