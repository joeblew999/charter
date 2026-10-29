// A project on Cloudflare: what its cloudflare.config.ts takes from charter (the compatibility date, its
// optional secrets, and its Workers Rate Limiting bindings, one for each limit its spec declares,
// x-rate-limit: charter/ratelimit, charter-lib/ratelimit, each counting in a namespace named from the
// Worker's name), its deploy with its secrets, and its migrations, on Cloudflare and on the local
// database of cf dev.
//   env: { ...rateLimits(spec, name, bindings.rateLimit) }
import { existsSync, mkdtempSync, readdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import type { Config } from "./config.ts";
import { output, run } from "./run.ts";

/** The Workers runtime's compatibility date every project runs on: the one charter's projects are
 * tested with, here so that it moves for all of them at once.
 *   worker: { compatibilityDate, ... } */
export const compatibilityDate = "2026-09-25";

/** The optional secrets the environment has, each a secret binding (secret: cf/config's
 * bindings.secret): a deploy sets those, cf dev takes them from the environment, and a Worker without
 * one does without what it is for.
 *   env: { ...secretsFromEnv(optionalSecrets, bindings.secret) } */
export function secretsFromEnv<B>(names: readonly string[], secret: () => B): Record<string, B> {
	return Object.fromEntries(names.filter(name => process.env[name]).map(name => [name, secret()]));
}

/** A limit as the spec declares it. */
type Limit = { binding: string; limit: number; period: 10 | 60 };

/** What cf/config's bindings.rateLimit takes. */
export interface RateLimitOptions {
	namespace: string;
	simple: { limit: number; period: 10 | 60 };
}

/**
 * The counter of a Worker's binding: a positive integer, as Cloudflare has it, which must be unique in
 * the account. It is named from the Worker and the binding (FNV-1a), so no project numbers its own:
 * two Workers never share a counter by mistake, and a Worker keeps its counter across deploys.
 */
export function namespaceOf(worker: string, binding: string): string {
	let hash = 0x811c9dc5;
	for (const byte of new TextEncoder().encode(`${worker}/${binding}`)) hash = Math.imul(hash ^ byte, 0x01000193) >>> 0;
	return String((hash % 1_000_000_000) + 1);
}

/** Each limit the spec declares, as a binding of its name: rateLimit is cf/config's bindings.rateLimit. */
export function rateLimits<B>(spec: { paths?: object }, worker: string, rateLimit: (options: RateLimitOptions) => B): Record<string, B> {
	const declared = Object.values(spec.paths ?? {})
		.flatMap(item => Object.values(item as Record<string, { "x-rate-limit"?: Limit }>))
		.flatMap(op => (op && typeof op === "object" && op["x-rate-limit"]) || []);
	return Object.fromEntries(declared.map(({ binding, limit, period }) => [binding, rateLimit({ namespace: namespaceOf(worker, binding), simple: { limit, period } })]));
}

/** migrateLocal applies migrations/*.sql, each once, to the local D1 database of the dev server on
 * port, through cf dev's local explorer. */
export async function migrateLocal(project: Pick<Config, "worker">, port: number, dir = "."): Promise<void> {
	const endpoint = `http://localhost:${port}/cdn-cgi/local/explorer/api/d1/database/DB-${project.worker}/raw`;
	const sql = async (statement: string): Promise<unknown> => {
		const reply = await fetch(endpoint, { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify({ sql: statement }) });
		const body = (await reply.json()) as { success: boolean; errors: unknown[]; result?: { results?: { rows?: unknown[][] } }[] };
		if (!body.success) throw new Error(`local D1 refused (HTTP ${reply.status} ${JSON.stringify(body.errors)}): ${statement}`);
		return body.result?.at(-1)?.results?.rows?.[0]?.[0];
	};
	await sql("CREATE TABLE IF NOT EXISTS _local_migrations (name TEXT PRIMARY KEY, applied_at TEXT DEFAULT CURRENT_TIMESTAMP)");
	for (const name of readdirSync(join(dir, "migrations")).filter(file => file.endsWith(".sql")).sort()) {
		if (Number(await sql(`SELECT count(*) FROM _local_migrations WHERE name = '${name}'`)) > 0) continue;
		await sql(readFileSync(join(dir, "migrations", name), "utf8"));
		await sql(`INSERT INTO _local_migrations (name) VALUES ('${name}')`);
		console.log("applied", name);
	}
}

/** deploy deploys the Worker with its secrets from the environment (never printed: they reach cf in a
 * file only this user can read, removed after), then applies migrations/ to its D1 database, if the
 * project has migrations. `mode` deploys another of the config's modes (cf deploy --mode): the Worker
 * <worker>-<mode>, as cloudflare.config.ts names it, with a database of its own. */
export function deploy(project: Pick<Config, "worker" | "secrets" | "optionalSecrets">, mode?: string): void {
	const secrets: Record<string, string> = {};
	for (const name of project.secrets ?? []) {
		const value = process.env[name];
		if (!value) throw new Error(`${name} is not set: the Worker needs it (fnox exec -- npm run deploy)`);
		secrets[name] = value;
	}
	for (const name of project.optionalSecrets ?? []) {
		const value = process.env[name];
		if (value) secrets[name] = value;
	}
	const args = ["deploy", ...(mode ? ["--mode", mode] : [])];
	const folder = mkdtempSync(join(tmpdir(), "charter-secrets-"));
	try {
		if (Object.keys(secrets).length > 0) {
			const file = join(folder, "secrets.json");
			writeFileSync(file, JSON.stringify(secrets), { mode: 0o600 });
			args.push("--secrets-file", file);
		}
		run("cf", args);
	} finally {
		rmSync(folder, { recursive: true, force: true });
	}
	if (!existsSync("migrations")) return;
	const database = `${project.worker}${mode ? `-${mode}` : ""}-db`;
	const found = (JSON.parse(output("cf", ["d1", "list", "--name", database, "--per-page", "100"])) as { name: string; uuid: string }[]).find(d => d.name === database);
	if (!found) throw new Error(`no D1 database ${database} after the deploy`);
	run("cf", ["d1", "migrations", "apply", found.uuid, "--dir", "migrations"]);
}

