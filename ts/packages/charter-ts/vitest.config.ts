import { defineConfig } from "vitest/config";

// Unit tests (test/*.test.ts) run in Node: follow() is plain TypeScript with no Workers APIs.
// The Workers that use it are tested live (notes/test/live-test.ts here; go/notes/test/soak.mjs).
export default defineConfig({ test: { include: ["test/**/*.test.ts"], environment: "node" } });
