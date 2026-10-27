#!/usr/bin/env node

// Passes the compiled test files to `node --test` explicitly: a directory
// argument is loaded as a module, and Node 20 does not expand --test globs.
import { spawnSync } from "node:child_process";
import { readdir } from "node:fs/promises";
import { join } from "node:path";
import { fileURLToPath } from "node:url";

const dir = fileURLToPath(new URL("../dist/tests/", import.meta.url));
const files = (await readdir(dir, { recursive: true }))
  .filter((name) => name.endsWith(".test.js"))
  .sort()
  .map((name) => join(dir, name));

if (files.length === 0) {
  console.error("No compiled tests found in dist/tests; run `npm run build` first.");
  process.exit(1);
}

const result = spawnSync(process.execPath, ["--test", ...files], { stdio: "inherit" });
process.exit(result.status ?? 1);
