# The Python client against a running Worker: who may do what, as the contract says it (README.md).
import os
import sys

from PACKAGE.client import CharterNotesApi
from PACKAGE.core.api_error import ApiError
from PACKAGE.environment import CharterNotesApiEnvironment

base = os.environ["BASE"]
there = CharterNotesApiEnvironment(base=base, production=base.replace("http", "ws", 1))


def client(token=None):
    return CharterNotesApi(environment=there, token=token)


def refused(what, call, status):
    try:
        call()
    except ApiError as e:
        if e.status_code == status:
            return
        sys.exit(f"{what}: HTTP {e.status_code}, want {status}: {e.body}")
    sys.exit(f"{what}: let through, want {status}")


# A client sends nothing without a token, so nobody is a token nobody issued.
refused("nobody writing", lambda: client("not-a-token").notes.create(body="from Python, as nobody"), 401)
refused("a reader writing", lambda: client(os.environ["READ_TOKEN"]).notes.create(body="from Python, as a reader"), 403)
note = client(os.environ["WRITE_TOKEN"]).notes.create(body="from the Python client")
newest = next(iter(client(os.environ["READ_TOKEN"]).notes.list(limit=1)))
if newest.id != note.id or newest.body != "from the Python client":
    sys.exit(f"what was written is not what is read back: {newest}")
print("python: nobody and a reader are refused, a writer writes, and it is read back")
