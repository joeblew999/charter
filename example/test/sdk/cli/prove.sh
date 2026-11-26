#!/bin/sh
# The command-line client against a running Worker: who may do what, as the contract says it (README.md). The
# client reads where the Worker is from NOTES_BASE_URL and its token from NOTES_TOKEN.
set -u
export NOTES_BASE_URL="$BASE"

refused() { # what, token, the status wanted in what the client says
  out=$(NOTES_TOKEN="$2" notes notes create --body "from the CLI, $1" 2>&1) && { echo "$1: let through, want $3"; exit 1; }
  echo "$out" | grep -q "$3" || { echo "$1: want $3, the client says: $out"; exit 1; }
}

# A client sends nothing without a token, so nobody is a token nobody issued.
refused "as nobody" "not-a-token" 401
refused "as a reader" "$READ_TOKEN" 403
NOTES_TOKEN="$WRITE_TOKEN" notes notes create --body "from the command-line client" >/dev/null || { echo "a writer was refused"; exit 1; }
NOTES_TOKEN="$READ_TOKEN" notes notes list --limit 1 | grep -q "from the command-line client" || { echo "what was written is not read back"; exit 1; }
echo "cli: nobody and a reader are refused, a writer writes, and it is read back"
