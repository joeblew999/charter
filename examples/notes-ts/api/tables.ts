// Written by `charter-ts spec` from migrations/*.sql: don't edit. Change the schema with a new
// migration, then npm run spec. The tables as Drizzle has them, for typed queries on D1:
//   drizzle(env.DB).select().from(table)
import { sql } from "drizzle-orm";
import { integer, sqliteTable, text } from "drizzle-orm/sqlite-core";

/** The table notes. */
export const notes = sqliteTable("notes", {
	id: integer("id").primaryKey({ autoIncrement: true }),
	body: text("body").notNull(),
	created_at: text("created_at").notNull().default(sql`CURRENT_TIMESTAMP`),
});
