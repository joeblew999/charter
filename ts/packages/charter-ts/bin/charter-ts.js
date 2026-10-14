#!/usr/bin/env node
// The charter-ts command: dist/cli.js, which npm run build writes. This file is what npm links, so it
// is there before the build is (a fresh clone of the repo: npm ci links, then builds).
import "../dist/cli.js";
