// intl's Intl outside a Worker: workerd serving intl-worker.mjs's goIntl over HTTP, for intl/workerd (a POST of
// {method, args}, or a message tree for intl.mjs's message). intl-workerd.capnp runs it.
import { goIntl } from "./intl-worker.mjs";
import * as intl from "./intl.mjs";
export default {
  async fetch(req) {
    if (req.method !== "POST") return new Response("ok");
    const { method, args, tree } = await req.json();
    if (method === "tree") return new Response(intl.message(args[0], tree, args.slice(1)));
    if (typeof goIntl[method] !== "function") return new Response("no method " + method, { status: 400 });
    return new Response(String(goIntl[method](...args)));
  },
};
