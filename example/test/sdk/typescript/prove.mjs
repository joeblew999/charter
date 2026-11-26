// The TypeScript client (as a release ships it: compiled) against a running Worker: who may do what, as the
// contract says it (README.md). SDK is where the client was generated.
const { NotesClient, NotesError } = await import(process.env.SDK);

const client = (token) => new NotesClient({ baseUrl: process.env.BASE, token });

async function refused(what, call, status) {
  try {
    await call();
  } catch (e) {
    if (e instanceof NotesError && e.statusCode === status) return;
    throw new Error(`${what}: ${e.statusCode ?? e}, want ${status}`);
  }
  throw new Error(`${what}: let through, want ${status}`);
}

// A client sends nothing without a token, so nobody is a token nobody issued.
await refused("nobody writing", () => client("not-a-token").notes.create({ body: "from TypeScript, as nobody" }), 401);
await refused("a reader writing", () => client(process.env.READ_TOKEN).notes.create({ body: "from TypeScript, as a reader" }), 403);
const note = await client(process.env.WRITE_TOKEN).notes.create({ body: "from the TypeScript client" });
const page = await client(process.env.READ_TOKEN).notes.list({ limit: 1 });
const newest = page.data[0];
if (newest.id !== note.id || newest.body !== "from the TypeScript client") {
  throw new Error(`what was written is not what is read back: ${JSON.stringify(newest)}`);
}
console.log("typescript: nobody and a reader are refused, a writer writes, and it is read back");
