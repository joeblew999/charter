// docker.ts and bin/cloud/docker: containers that reach the internet through a proxy on the machine.
//   node --test src/*.test.ts   (npm run check -w charter-ts runs them)
import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { chmodSync, mkdtempSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { test } from "node:test";
import { containerEnv, proxyCA } from "./docker.ts";
import { packageDir } from "./package.ts";

const has = (...files: string[]) => (file: string) => files.includes(file);

test("proxyCA: a loopback proxy with its CA, and nothing else", () => {
	assert.equal(proxyCA({ HTTPS_PROXY: "http://127.0.0.1:32915" }, has("/root/.ccr/ca-bundle.crt")), "/root/.ccr/ca-bundle.crt");
	assert.equal(proxyCA({ https_proxy: "http://localhost:8080", NODE_EXTRA_CA_CERTS: "/etc/ca.pem" }, has("/etc/ca.pem")), "/etc/ca.pem");
	assert.equal(proxyCA({}, has("/root/.ccr/ca-bundle.crt")), undefined, "no proxy: a Mac, a GitHub runner");
	assert.equal(proxyCA({ HTTPS_PROXY: "http://proxy.corp:3128" }, has("/root/.ccr/ca-bundle.crt")), undefined, "a proxy containers can reach themselves");
	assert.equal(proxyCA({ HTTPS_PROXY: "http://127.0.0.1:32915" }, has()), undefined, "no CA to give them");
});

test("containerEnv: charter-ts's docker first on the PATH only behind a loopback proxy", () => {
	const env = containerEnv({ HTTPS_PROXY: "http://127.0.0.1:1", PATH: "/usr/bin" }, has("/root/.ccr/ca-bundle.crt"));
	assert.match(env.PATH ?? "", /bin[\\/]cloud[:;]\/usr\/bin$/);
	assert.equal(env.CHARTER_PROXY_CA, "/root/.ccr/ca-bundle.crt");
	assert.deepEqual(containerEnv({ PATH: "/usr/bin" }, has("/root/.ccr/ca-bundle.crt")), {});
});

test("bin/cloud/docker: run gets the host's network, the proxy and the CA; everything else is unchanged", { skip: process.platform === "win32" }, () => {
	// A stand-in for the real docker, after the wrapper on the PATH: it prints what it was given.
	const fake = mkdtempSync(join(tmpdir(), "docker-"));
	writeFileSync(join(fake, "docker"), '#!/bin/sh\nprintf "%s\\n" "$@"\n');
	chmodSync(join(fake, "docker"), 0o755);
	// A real PATH, with the stand-in ahead of any docker installed.
	const path = `${fake}:/usr/local/bin:/usr/bin:/bin`;
	const cloud = containerEnv({ HTTPS_PROXY: "http://127.0.0.1:1", PATH: path }, () => true);
	// The program as the PATH would find it: charter-ts's docker when containerEnv put it first.
	const docker = (env: Record<string, string>, ...args: string[]) =>
		spawnSync(env.CHARTER_PROXY_CA ? join(packageDir, "bin", "cloud", "docker") : join(fake, "docker"), args, { env: { ...process.env, HTTPS_PROXY: "http://127.0.0.1:1", NO_PROXY: "localhost", PATH: path, ...env }, encoding: "utf8" }).stdout.trim().split("\n");

	const run = docker(cloud, "run", "--rm", "image", "/fern/config.json");
	assert.deepEqual(run.slice(0, 3), ["run", "--network", "host"]);
	assert.ok(run.includes("HTTPS_PROXY=http://127.0.0.1:1") && run.includes("NO_PROXY=localhost"));
	assert.ok(run.includes(`${cloud.CHARTER_PROXY_CA}:/etc/charter-proxy-ca.crt:ro`) && run.includes("NODE_EXTRA_CA_CERTS=/etc/charter-proxy-ca.crt"));
	assert.deepEqual(run.slice(-3), ["--rm", "image", "/fern/config.json"], "the caller's arguments, last and unchanged");

	assert.deepEqual(docker(cloud, "compose", "up", "-d"), ["compose", "up", "-d"], "not a run: unchanged");
	assert.deepEqual(docker({ PATH: path }, "run", "image"), ["run", "image"], "no charter-ts docker on the PATH: the real one, untouched");
});
