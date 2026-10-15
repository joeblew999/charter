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
	/** Both specs from the contract (charter-ts spec writes them into fern/): in TypeScript, the two
	 * generators; in Go, the package that writes them. */
	specs: Specs | GoSpecs;
	/** The server the specs name, if not `url`. */
	specServer?: string;
	/** A folder the generated TypeScript SDK is copied into before lint, the workerd test and a deploy,
	 * for a Worker that runs the SDK itself (src/client). */
	sdkInWorker?: string;
	/** More modes to deploy after the default one (cf deploy --mode): the same code as another Worker. */
	deployModes?: readonly string[];
	/** charter-ts test:workerd: the Worker under workerd (cf dev) on a free port, and these tests against it. */
	test: Tests;
	/** charter-ts test:native, in Go: the API run natively (go run .) on a free port, and these tests against it. */
	native?: Tests;
	/** A Go project: its lint, its build with TinyGo, what it commits that is generated. */
	go?: GoConfig;
	/** charter-ts live-test: tests against the deployed Worker (url). */
	live?: (context: LiveContext) => void | Promise<void>;
}

export interface Tests {
	/** A path that answers once the server is up. Default /api/hello. */
	ready?: string;
	/** More environment for the server, given its port and the free ones the tests may serve on. */
	env?: (ports: { port: number; extra: number; extras: Extras }) => Record<string, string>;
	/** What the tests serve beside the server for its whole run (a webhook's receiver): each started
	 * before it, with the server's environment, waited for at its `ready` URL, and stopped after. */
	serve?: (ports: { port: number; extra: number; extras: Extras }) => Helper[];
	run: (context: TestContext) => void | Promise<void>;
}

/** Go's specs: go run <package> [-check] fern/openapi.json fern/asyncapi.json <server>. */
export interface GoSpecs {
	go: string;
}

export interface GoConfig {
	/** The folders gofmt checks beside the root's Go files. Default api, cmd. */
	lint?: string[];
	/** The TinyGo build of the Worker (build/app.wasm). */
	build?: {
		/** The starting heap, so an ordinary request never collects. Default 8. */
		heapMB?: number;
		/** TinyGo's optimisation: z and s for size, 1 and 2 for speed. Default z. */
		opt?: "z" | "s" | "1" | "2";
		/** The stack per goroutine (Huma overflows TinyGo's 64 KB). Default 128kb. */
		stack?: string;
		/** Fail over this many bytes gzipped (Workers Free allows 3 MB). Default 3000000. */
		maxBytes?: number;
	};
	/** gsx components whose generated *.x.go are committed: lint fails if they are stale. */
	gsx?: boolean;
	/** The Go SDK committed in sdk/go: lint fails if the specs changed since it was made. */
	sdk?: boolean;
}

/** A program the tests serve beside the server, answering at `ready` once it is up. */
export interface Helper {
	program: string;
	args: string[];
	ready: string;
}

/** Three free ports, all different. */
export type Extras = readonly [number, number, number];

export interface TestContext {
	/** The local Worker, http://localhost:<port>. */
	origin: string;
	port: number;
	/** Another free port, for something a test serves (a test issuer of tokens): extras[0]. */
	extra: number;
	/** Three free ports, all different, for what the tests serve: one each (an issuer of tokens, a
	 * webhook's receiver). The first is `extra`. */
	extras: Extras;
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
