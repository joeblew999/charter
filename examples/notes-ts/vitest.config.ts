import { defineConfig } from "vitest/config";

// Unit tests (test/*.test.ts) run in Node: follow() is plain TypeScript with no Workers APIs.
// The Worker itself is tested live (../notes-go/test/live-test.mjs and soak.mjs).
export default defineConfig({ test: { include: ["test/**/*.test.ts"], environment: "node" } });
