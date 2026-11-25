# intl's Intl outside a Worker: workerd serving intl-workerd.mjs, with the catalogs (mise-intl workerd runs it on the
# port pitchfork gives it, --socket-addr http=127.0.0.1:$PORT).
using Workerd = import "/workerd/workerd.capnp";
const config :Workerd.Config = (
  services = [(name = "main", worker = (
    modules = [
      (name = "main.mjs", esModule = embed "intl-workerd.mjs"),
      (name = "intl-worker.mjs", esModule = embed "intl-worker.mjs"),
      (name = "intl.mjs", esModule = embed "intl.mjs"),
      (name = "intl-messages.mjs", esModule = embed "intl-messages.mjs"),
    ],
    compatibilityDate = "2026-09-11"))],
  sockets = [(name = "http", address = "127.0.0.1:8749", http = (), service = "main")],
);
