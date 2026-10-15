// The procedures of an oRPC router or contract, in its order: what serve, mcp and auth read from it.
import type { Procedure } from "@orpc/server";

export type AnyProcedure = Procedure<any, any, any, any, any>;

/** proceduresOf is every procedure of a router (or a contract) and of the routers inside it. */
export function proceduresOf(router: unknown): AnyProcedure[] {
	const found: AnyProcedure[] = [];
	const walk = (node: unknown): void => {
		if (node && typeof node === "object" && "~orpc" in node) found.push(node as AnyProcedure);
		else if (node && typeof node === "object") Object.values(node).forEach(walk);
	};
	walk(router);
	return found;
}
