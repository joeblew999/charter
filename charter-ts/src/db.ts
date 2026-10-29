// A project's database code, typed, from its schema: migrations/*.sql, the one place the tables are
// said. npm run spec writes it beside the specs, and check fails when it is stale.
//   Go (charter.config.ts: db: "sqlc"): the queries in api/queries.sql as functions over charter/d1,
//   api/queries.go. sqlc works out each one's parameters and columns against the schema; charter-ts
//   writes the Go, with no database/sql (charter/d1 says why).
//   TypeScript (db: "drizzle"): the tables as Drizzle has them, api/tables.ts, read from the schema
//   as SQLite itself has it once the migrations ran; queries are Drizzle's, on drizzle(env.DB).
import { spawnSync } from "node:child_process";
import { existsSync, mkdtempSync, readdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, relative, resolve } from "node:path";
import { DatabaseSync } from "node:sqlite";
import { miseTool } from "./pins.ts";
import { output } from "./run.ts";

/** A column or a parameter as sqlc has it (its codegen request, in JSON). */
interface SqlcColumn {
	name: string;
	not_null: boolean;
	is_array?: boolean;
	is_sqlc_slice?: boolean;
	type: { name: string };
	table?: { name: string } | null;
}

interface SqlcQuery {
	name: string;
	cmd: string;
	text: string;
	comments?: string[];
	columns: SqlcColumn[];
	params: { number: number; column: SqlcColumn }[];
}

export interface SqlcRequest {
	catalog: { schemas: { tables?: { rel: { name: string }; columns: SqlcColumn[] }[] }[] };
	queries: SqlcQuery[];
}

/** analyze runs sqlc on the project's schema and queries, and returns what it worked out. */
function analyze(): SqlcRequest {
	const dir = mkdtempSync(join(tmpdir(), "charter-sqlc-"));
	try {
		writeFileSync(join(dir, "sqlc.yaml"), [
			'version: "2"',
			"sql:",
			"  - engine: sqlite",
			// sqlc reads both relative to its config, even a path that is absolute.
			`    schema: ${JSON.stringify(relative(dir, resolve("migrations")))}`,
			`    queries: ${JSON.stringify(relative(dir, resolve("api", "queries.sql")))}`,
			"    gen:",
			"      json:",
			"        out: out",
			"",
		].join("\n"));
		// The sqlc that works out the Go queries' types, at the version mise.toml pins (pins.ts).
		output("mise", ["exec", miseTool("aqua:sqlc-dev/sqlc"), "--", "sqlc", "generate", "-f", join(dir, "sqlc.yaml")]);
		return JSON.parse(readFileSync(join(dir, "out", "codegen_request.json"), "utf8")) as SqlcRequest;
	} finally {
		rmSync(dir, { recursive: true, force: true });
	}
}

const initialisms = new Set(["id", "url", "uri", "api", "http", "json", "uuid", "sql", "ip", "html", "css"]);
const words = (name: string) => name.split(/[^A-Za-z0-9]+/).filter(Boolean);
const capital = (word: string) => (initialisms.has(word.toLowerCase()) ? word.toUpperCase() : word[0]!.toUpperCase() + word.slice(1));
/** A Go name others may use: created_at is CreatedAt, user_id UserID. */
const exported = (name: string) => words(name).map(capital).join("");
const keywords = new Set(["break", "case", "chan", "const", "continue", "default", "defer", "else", "fallthrough", "for", "func", "go", "goto", "if", "import", "interface", "map", "package", "range", "return", "select", "struct", "switch", "type", "var"]);
/** A Go name of the function's own: created_at is createdAt, id id. */
const local = (name: string) => {
	const [first = "arg", ...rest] = words(name);
	const camel = first.toLowerCase() + rest.map(capital).join("");
	return keywords.has(camel) ? `${camel}_` : camel;
};

/** The Go type of a column, by SQLite's rules of affinity, with dates as the text D1 gives; a column
 * that may be NULL is a pointer. */
function goType(column: SqlcColumn): string {
	if (column.is_sqlc_slice || column.is_array) throw new Error(`api/queries.sql: ${column.name} is a list (sqlc.slice): charter-ts writes queries of single values`);
	const name = column.type.name.toLowerCase();
	const base = name.includes("int") ? "int64"
		: /char|clob|text|date|time/.test(name) ? "string"
		: /real|floa|doub|numeric|decimal/.test(name) ? "float64"
		: name.includes("bool") ? "bool"
		: name.includes("blob") ? "[]byte"
		: "any";
	return column.not_null || base === "[]byte" || base === "any" ? base : `*${base}`;
}

const goField = (column: SqlcColumn) => `\t${exported(column.name)} ${goType(column)} \`json:"${column.name}"\``;

/** goQueries is api/queries.go for what sqlc worked out: a function for each query, over charter/d1. */
export function goQueries(request: SqlcRequest, pkg: string): string {
	const tables = new Map(request.catalog.schemas.flatMap(schema => schema.tables ?? []).map(table => [table.rel.name, table.columns]));
	const types: string[] = [];
	const made = new Set<string>();
	/** The row type of a query: its table's, when it has every column of one table, in order. */
	const rowOf = (query: SqlcQuery): string => {
		const table = query.columns[0]?.table?.name;
		const columns = table ? tables.get(table) : undefined;
		const whole = columns && columns.length === query.columns.length && query.columns.every((c, i) => c.table?.name === table && c.name === columns[i]!.name);
		const name = whole ? `${exported(table!)}Row` : `${query.name}Row`;
		if (!made.has(name)) {
			made.add(name);
			types.push(whole ? `// ${name} is a row of the table ${table}.` : `// ${name} is a row of ${query.name}.`, `type ${name} struct {`, ...(whole ? columns! : query.columns).map(goField), "}", "");
		}
		return name;
	};
	const functions = request.queries.map(query => {
		if (![":one", ":many", ":exec"].includes(query.cmd)) throw new Error(`api/queries.sql: ${query.name} is ${query.cmd}: charter-ts writes :one, :many and :exec`);
		const taken = new Set(["db"]);
		const params = [...query.params].sort((a, b) => a.number - b.number).map(({ column }) => {
			let name = local(column.name);
			for (let n = 2; taken.has(name); n++) name = `${local(column.name)}${n}`;
			taken.add(name);
			const type = goType(column);
			return { name, type, arg: type.startsWith("*") ? `d1.OrNull(${name})` : name };
		});
		const signature = ["db d1.DB", ...params.map(p => `${p.name} ${p.type}`)].join(", ");
		const args = ["db", `q${query.name}`, ...params.map(p => p.arg)].join(", ");
		// The comment above a query is its doc when it names it, as Go's do; what comes before the first
		// query (the file's own) does not.
		const own = query.comments?.map(line => line.trim()) ?? [];
		const about = own[0]?.startsWith(`${query.name} `) ? own.map(line => `// ${line}`) : [`// ${query.name} runs the query ${query.name} of api/queries.sql.`];
		const lines = [`const q${query.name} = ${JSON.stringify(query.text.trim())}`, "", ...about];
		if (query.cmd === ":exec") return [...lines, `func ${query.name}(${signature}) error {`, `\treturn d1.Exec(${args})`, "}"].join("\n");
		const many = query.cmd === ":many";
		if (query.columns.length === 1) {
			// One column: its values, not rows of one field.
			const [column] = query.columns as [SqlcColumn];
			const type = goType(column), row = rowOf(query), field = exported(column.name);
			return many
				? [...lines, `func ${query.name}(${signature}) ([]${type}, error) {`, `\trows, err := d1.Query[${row}](${args})`, `\tvalues := make([]${type}, len(rows))`, "\tfor i, row := range rows {", `\t\tvalues[i] = row.${field}`, "\t}", "\treturn values, err", "}"].join("\n")
				: [...lines, `func ${query.name}(${signature}) (${type}, error) {`, `\trow, err := d1.One[${row}](${args})`, `\treturn row.${field}, err`, "}"].join("\n");
		}
		if (query.columns.length === 0) throw new Error(`api/queries.sql: ${query.name} is ${query.cmd} but returns no columns: make it :exec, or add RETURNING`);
		const row = rowOf(query);
		return [...lines, `func ${query.name}(${signature}) (${many ? `[]${row}` : row}, error) {`, `\treturn d1.${many ? "Query" : "One"}[${row}](${args})`, "}"].join("\n");
	});
	const source = [
		"// Code generated by charter-ts spec from migrations/*.sql and api/queries.sql, with sqlc. DO NOT EDIT.",
		"// Change those, then npm run spec.",
		"",
		"//go:build js && wasm",
		"",
		`package ${pkg}`,
		"",
		'import "github.com/joeblew999/charter/d1"',
		"",
		...types,
		functions.join("\n\n"),
		"",
	].join("\n");
	return gofmt(source);
}

/** gofmt is Go source as gofmt writes it. */
function gofmt(source: string): string {
	const result = spawnSync("gofmt", [], { input: source, encoding: "utf8" });
	if (result.status !== 0) throw new Error(`the Go written for api/queries.sql does not format: ${result.stderr}\n${source}`);
	return result.stdout;
}

/** A column as SQLite itself has it (PRAGMA table_info). */
interface SqliteColumn {
	name: string;
	type: string;
	notnull: number;
	dflt_value: string | null;
	pk: number;
}

/** drizzleTables is api/tables.ts: each table the migrations make, as Drizzle has it. */
export function drizzleTables(migrations: string[]): string {
	const db = new DatabaseSync(":memory:");
	try {
		for (const sql of migrations) db.exec(sql);
		const tables = db.prepare("SELECT name, sql FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite\\_%' ESCAPE '\\' AND name NOT LIKE '\\_cf\\_%' ESCAPE '\\' AND name <> 'd1_migrations' ORDER BY name").all() as { name: string; sql: string }[];
		const builders = new Set<string>();
		let defaults = false;
		const blocks = tables.map(({ name, sql }) => {
			const columns = db.prepare(`PRAGMA table_info(${JSON.stringify(name)})`).all() as unknown as SqliteColumn[];
			const unique = new Set((db.prepare(`PRAGMA index_list(${JSON.stringify(name)})`).all() as { name: string; unique: number; origin: string }[])
				.filter(index => index.unique && index.origin === "u")
				.map(index => db.prepare(`PRAGMA index_info(${JSON.stringify(index.name)})`).all() as { name: string }[])
				.filter(columns => columns.length === 1)
				.map(([column]) => column!.name));
			const keys = columns.filter(column => column.pk > 0).sort((a, b) => a.pk - b.pk);
			const fields = columns.map(column => {
				const type = column.type.toLowerCase();
				const builder = type.includes("int") ? "integer" : /char|clob|text|date|time/.test(type) ? "text" : /real|floa|doub/.test(type) ? "real" : type.includes("blob") || type === "" ? "blob" : "numeric";
				builders.add(builder);
				let field = `${builder}(${JSON.stringify(column.name)})`;
				if (keys.length === 1 && column.pk) field += /autoincrement/i.test(sql) ? ".primaryKey({ autoIncrement: true })" : ".primaryKey()";
				else if (column.notnull) field += ".notNull()";
				if (unique.has(column.name)) field += ".unique()";
				if (column.dflt_value !== null) {
					defaults = true;
					field += `.default(sql\`${column.dflt_value.replaceAll("\\", "\\\\").replaceAll("`", "\\`").replaceAll("${", "\\${")}\`)`;
				}
				return `\t${/^[A-Za-z_$][\w$]*$/.test(column.name) ? column.name : JSON.stringify(column.name)}: ${field},`;
			});
			const composite = keys.length > 1 ? `, table => [primaryKey({ columns: [${keys.map(k => `table[${JSON.stringify(k.name)}]`).join(", ")}] })]` : "";
			if (composite) builders.add("primaryKey");
			const ident = words(name).map((word, i) => (i === 0 ? word.toLowerCase() : capital(word))).join("");
			return [`/** The table ${name}. */`, `export const ${ident} = sqliteTable(${JSON.stringify(name)}, {`, ...fields, `}${composite});`].join("\n");
		});
		builders.add("sqliteTable");
		return [
			"// Written by `charter-ts spec` from migrations/*.sql: don't edit. Change the schema with a new",
			"// migration, then npm run spec. The tables as Drizzle has them, for typed queries on D1:",
			"//   drizzle(env.DB).select().from(table)",
			...(defaults ? ['import { sql } from "drizzle-orm";'] : []),
			`import { ${[...builders].sort().join(", ")} } from "drizzle-orm/sqlite-core";`,
			"",
			blocks.join("\n\n"),
			"",
		].join("\n");
	} finally {
		db.close();
	}
}

/** The migrations, in the order they are applied. */
function migrations(): string[] {
	if (!existsSync("migrations")) return [];
	return readdirSync("migrations").filter(name => name.endsWith(".sql")).sort().map(name => readFileSync(join("migrations", name), "utf8"));
}

/** writeDatabaseCode writes the project's database code (db: "sqlc" or "drizzle"); with check it
 * writes nothing, and fails if what is there is not what the schema gives. */
export function writeDatabaseCode(db: "sqlc" | "drizzle", check: boolean): void {
	let file: string, text: string;
	if (db === "sqlc") {
		if (!existsSync("go.mod")) throw new Error(`db: "sqlc" writes Go: this project has no go.mod (a TypeScript project takes db: "drizzle")`);
		if (!existsSync(join("api", "queries.sql"))) throw new Error(`db: "sqlc" writes the queries of api/queries.sql, and there is none`);
		const pkg = readdirSync("api").filter(name => name.endsWith(".go")).map(name => /^package (\w+)/m.exec(readFileSync(join("api", name), "utf8"))?.[1]).find(Boolean) ?? "api";
		[file, text] = [join("api", "queries.go"), goQueries(analyze(), pkg)];
	} else {
		[file, text] = [join("api", "tables.ts"), drizzleTables(migrations())];
	}
	if (!check) writeFileSync(file, text);
	else if (!existsSync(file) || readFileSync(file, "utf8") !== text) throw new Error(`${file} is not what migrations/ gives: npm run spec`);
	console.log(check ? `${file} is what migrations/ gives` : `${file}: from migrations/${db === "sqlc" ? " and api/queries.sql (sqlc)" : ""}`);
}
