// The Worker Cloudflare runs: the API (api/handlers.ts).
import { handler } from "./api/handlers.ts";

export default {
	fetch: request => handler(request, {}),
} satisfies ExportedHandler;
