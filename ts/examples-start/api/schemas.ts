import { z } from "zod";

// What the API answers, as Zod schemas: the contract's outputs (api/contract.ts), and what the tests
// check every answer against (test/). Zod alone, so a Go twin's tests carry them as they are.

/** What GET /api/hello answers. */
export const hello = z.object({ message: z.string().meta({ examples: ["Hello from charter-start-ts"] }) });
