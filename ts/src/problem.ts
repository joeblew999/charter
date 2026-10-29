// How every API charter serves answers a refusal (docs/self/standard.md): RFC 9457 problem
// details, as Huma writes them in Go, so an API answers the same in either language.
//   {"title":"Unprocessable Entity","status":422,"detail":"validation failed","errors":[{"message":"expected number >= 1","location":"query.limit","value":0}]}

/** One thing a refused request got wrong: what, where (`query.limit`, `body.note.body`, `path.id`,
 * `body` for the whole body), and the value it had there. */
export interface ErrorDetail {
	message: string;
	location?: string;
	value?: unknown;
}

/** A problem's body. */
export interface Problem {
	title: string;
	status: number;
	detail: string;
	errors?: ErrorDetail[];
}

/** The media type of a problem. */
export const problemType = "application/problem+json";

/** The reason phrase of each status an API answers with: a problem's title. */
const titles: Record<number, string> = {
	400: "Bad Request", 401: "Unauthorized", 403: "Forbidden", 404: "Not Found", 405: "Method Not Allowed",
	406: "Not Acceptable", 408: "Request Timeout", 409: "Conflict", 410: "Gone", 412: "Precondition Failed",
	413: "Request Entity Too Large", 415: "Unsupported Media Type", 422: "Unprocessable Entity", 426: "Upgrade Required",
	429: "Too Many Requests", 500: "Internal Server Error", 501: "Not Implemented", 502: "Bad Gateway",
	503: "Service Unavailable", 504: "Gateway Timeout",
};

/** A status's reason phrase (a problem's title, a refusal's description in the specs). */
export const reasonOf = (status: number | string): string | undefined => titles[Number(status)];

/** A problem's body for a status and what went wrong. */
export function problemOf(status: number, detail: string, errors?: ErrorDetail[]): Problem {
	return { title: titles[status] ?? "Error", status, detail, ...(errors?.length ? { errors } : {}) };
}

/** A problem as the response. */
export function problem(status: number, detail: string, errors?: ErrorDetail[], headers: Record<string, string> = {}): Response {
	return new Response(JSON.stringify(problemOf(status, detail, errors)), { status, headers: { ...headers, "content-type": problemType } });
}
