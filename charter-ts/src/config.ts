// A project's charter.config.ts: what charter-ts needs to know about the project, typed, in one place.
//   import { defineConfig } from "charter-ts/config";
//   export default defineConfig({ worker: "my-api", url: "https://my-api.me.workers.dev", ... });
import type { Specs } from "./spec-files.ts";

export interface Config {
	/** The Worker's name, as in cloudflare.config.ts. Its D1 database is `${worker}-db`, locally `DB-${worker}`. */
	worker: string;
	/** Where the Worker is deployed (cf deploy prints it): the live test's target, and the server the specs name. */
	url: string;
	/** The port of charter-ts dev. */
	port: number;
	/** Secrets a deploy needs, every one, from the environment (fnox exec -- npm run deploy). */
	secrets?: readonly string[];
	/** Secrets a deploy sets where the environment has them. */
	optionalSecrets?: readonly string[];
	/** Throwaway values of the secrets for the local Worker: charter-ts dev and the workerd test. */
	localSecrets?: Readonly<Record<string, string>>;
	/** Both specs from the contract (charter-ts spec writes them into fern/). */
	specs: Specs;
	/** The server the specs name, if not `url`. */
	specServer?: string;
	/** A folder the generated TypeScript SDK is copied into before lint, the workerd test and a deploy,
	 * for a Worker that runs the SDK itself (src/client). */
	sdkInWorker?: string;
	/** More modes to deploy after the default one (cf deploy --mode): the same code as another Worker. */
	deployModes?: readonly string[];
	/** charter-ts test:workerd: the Worker under workerd (cf dev) on a free port, and these tests against it. */
	test: {
		/** A path that answers once the Worker is up. Default /api/hello. */
		ready?: string;
		/** More environment for the Worker, given its port and another free one the tests may serve on. */
		env?: (ports: { port: number; extra: number }) => Record<string, string>;
		run: (context: TestContext) => void | Promise<void>;
	};
	/** charter-ts live-test: tests against the deployed Worker (url). */
	live?: (context: LiveContext) => void | Promise<void>;
}

export interface TestContext {
	/** The local Worker, http://localhost:<port>. */
	origin: string;
	port: number;
	/** Another free port, for something a test serves (a test issuer of tokens). */
	extra: number;
	/** node <file> <args>, with the Worker's environment; throws if it fails. */
	node: (file: string, ...args: string[]) => void;
}

export interface LiveContext {
	url: string;
	/** node <file> <args>, with this environment (the secrets: fnox exec --); throws if it fails. */
	node: (file: string, ...args: string[]) => void;
}

/** defineConfig types a project's config. Fields of the project's own (another URL a test needs) stay typed too. */
export function defineConfig<T extends Config>(config: T): T {
	return config;
}
