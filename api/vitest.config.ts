import { defineConfig } from "vitest/config";

// Unit tests (test/*.test.ts) run in Node: follow() is plain TypeScript with no Workers APIs.
// The Worker itself is tested live (live-test.mjs, sse-soak.mjs).
export default defineConfig({ test: { include: ["test/**/*.test.ts"], environment: "node" } });
