// JSON Schemas as charter publishes them (the specs, the MCP tools), the same from a zod contract as
// Huma writes them from a Go one: zod says "an integer" with JavaScript's safe-integer bounds, which
// are no limit, and leaves a definition in $defs that nothing refers to once the root is inlined.
import { type JsonSchema, mapJsonSchemaRefs } from "@orpc/json-schema";

const safe = Number.MAX_SAFE_INTEGER;

/** A schema (or a document holding schemas) without the safe-integer bounds zod writes for an integer. */
export function withoutSafeBounds<T>(value: T): T {
	if (Array.isArray(value)) return value.map(withoutSafeBounds) as T;
	if (!value || typeof value !== "object") return value;
	const out: Record<string, unknown> = {};
	for (const [key, child] of Object.entries(value)) {
		if ((key === "minimum" && child === -safe) || (key === "maximum" && child === safe)) continue;
		out[key] = withoutSafeBounds(child);
	}
	return out as T;
}

/** A schema without the definitions in its $defs that nothing in it refers to. */
export function withoutUnusedDefs(schema: Record<string, unknown>): Record<string, unknown> {
	const defs = schema.$defs as Record<string, unknown> | undefined;
	if (!defs) return schema;
	const { $defs: _, ...rest } = schema;
	const used = new Set<string>();
	const visit = (value: unknown): void => {
		if (Array.isArray(value)) value.forEach(visit);
		else if (value && typeof value === "object") {
			for (const [key, child] of Object.entries(value)) {
				if (key === "$ref" && typeof child === "string" && child.startsWith("#/$defs/") && !used.has(child.slice(8))) {
					used.add(child.slice(8));
					visit(defs[child.slice(8)]);
				} else visit(child);
			}
		}
	};
	visit(rest);
	const kept = Object.fromEntries(Object.entries(defs).filter(([name]) => used.has(name)));
	return Object.keys(kept).length ? { ...rest, $defs: kept } : rest;
}

/** asComponent is a schema with its references to `$defs` (zod's) pointed at the document's
 * `components/schemas`, where a named schema goes in both specs. */
export const asComponent = (schema: JsonSchema): JsonSchema => mapJsonSchemaRefs(schema, ref => ref.replace(/^#\/\$defs\//, "#/components/schemas/"));
