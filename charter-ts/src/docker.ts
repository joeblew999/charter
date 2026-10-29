// Docker, readied for what charter-ts runs in containers (Fern's generators): running, and, where the
// machine reaches the internet through a proxy on itself with its own CA (a Claude cloud session),
// able to reach npm from inside a container. On a Mac or a GitHub runner the containers have the
// internet as they are, and this changes nothing but checking Docker is up.
import { spawnSync, spawn } from "node:child_process";
import { existsSync, openSync } from "node:fs";
import { join } from "node:path";
import { packageDir } from "./package.ts";

/** Where a Claude cloud session keeps the CA of its egress proxy. */
const cloudCA = "/root/.ccr/ca-bundle.crt";

/** The CA file of a proxy on this machine, when its internet goes through one: HTTPS_PROXY on a
 * loopback address, and its CA in NODE_EXTRA_CA_CERTS or where a Claude cloud session keeps it. */
export function proxyCA(env: NodeJS.ProcessEnv = process.env, exists: (path: string) => boolean = existsSync): string | undefined {
	const proxy = env.HTTPS_PROXY ?? env.https_proxy;
	if (!proxy) return undefined;
	let host: string;
	try {
		host = new URL(proxy).hostname;
	} catch {
		return undefined;
	}
	if (!["127.0.0.1", "localhost", "[::1]", "::1"].includes(host)) return undefined;
	for (const ca of [env.NODE_EXTRA_CA_CERTS, cloudCA]) if (ca && exists(ca)) return ca;
	return undefined;
}

/** The environment for a program that runs containers: with a proxy on this machine, charter-ts's
 * docker (bin/cloud/docker) first on the PATH and the proxy's CA named; otherwise nothing changes. */
export function containerEnv(env: NodeJS.ProcessEnv = process.env, exists: (path: string) => boolean = existsSync): Record<string, string> {
	const ca = proxyCA(env, exists);
	if (!ca) return {};
	const sep = process.platform === "win32" ? ";" : ":";
	return { PATH: `${join(packageDir, "bin", "cloud")}${sep}${env.PATH ?? ""}`, CHARTER_PROXY_CA: ca };
}

const dockerUp = () => spawnSync("docker", ["info"], { stdio: "ignore" }).status === 0;

/** dockerReady makes sure Docker answers before a container is run. Where it is installed and not
 * running and this user may start it (a cloud session, as root), it starts the daemon and waits; on
 * a desktop it says to start Docker, rather than let a generator fail halfway. */
export function dockerReady(): void {
	if (dockerUp()) return;
	const dockerd = spawnSync("sh", ["-c", "command -v dockerd"], { encoding: "utf8" }).stdout.trim();
	if (dockerd && process.getuid?.() === 0) {
		console.log(`Docker is not running: starting ${dockerd} (log: /tmp/charter-dockerd.log)`);
		const log = openSync("/tmp/charter-dockerd.log", "a");
		spawn(dockerd, [], { detached: true, stdio: ["ignore", log, log] }).unref();
		for (let i = 0; i < 60 && !dockerUp(); i++) spawnSync("sleep", ["1"]);
		if (dockerUp()) return;
		throw new Error("Docker did not start within 60 s: /tmp/charter-dockerd.log says why");
	}
	throw new Error("Docker is not running, and Fern's generators run in it: start Docker (Docker Desktop on a Mac), then run this again");
}
