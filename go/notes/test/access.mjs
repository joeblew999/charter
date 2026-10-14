// The headers that take a test through Cloudflare Access, when the Worker is behind it: the
// service token in <WORKER>_ACCESS_CLIENT_ID and <WORKER>_ACCESS_CLIENT_SECRET (WORKER: the first
// label of the origin's host, in capitals with _ for -; mise run access:token -- create <machine>
// fnox puts them in fnox). Empty when they are not set: the Worker is not behind Access.
export function accessHeaders(origin) {
  const prefix = new URL(origin).hostname.split(".")[0].replace(/[-. ]/g, "_").toUpperCase();
  const id = process.env[`${prefix}_ACCESS_CLIENT_ID`], secret = process.env[`${prefix}_ACCESS_CLIENT_SECRET`];
  return id && secret ? { "cf-access-client-id": id, "cf-access-client-secret": secret } : {};
}
