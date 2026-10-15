import { defineConfig } from "vitest/config";

// Unit tests (src/*.test.ts, beside the code they test, as Go's are) run in Node: the library is plain
// TypeScript with no Workers APIs.
// The Workers that use it are tested by their projects' checks (ts/examples-start, ts/examples-notes, ts/conformance).
export default defineConfig({ test: { include: ["src/**/*.test.ts"], environment: "node" } });
