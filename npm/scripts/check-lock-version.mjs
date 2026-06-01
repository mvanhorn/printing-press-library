#!/usr/bin/env node

import { readFile } from "node:fs/promises";

const [pkgRaw, lockRaw] = await Promise.all([
  readFile(new URL("../package.json", import.meta.url), "utf8"),
  readFile(new URL("../package-lock.json", import.meta.url), "utf8"),
]);

const pkg = JSON.parse(pkgRaw);
const lock = JSON.parse(lockRaw);
const rootPackage = lock.packages?.[""];
const lockVersions = [
  ["package-lock.json version", lock.version],
  ["package-lock.json packages[\"\"] version", rootPackage?.version],
];

const mismatches = lockVersions.filter(([, version]) => version !== pkg.version);
if (mismatches.length > 0) {
  for (const [label, version] of mismatches) {
    console.error(`${label} is ${version ?? "missing"}, expected ${pkg.version}`);
  }
  console.error("Run `npm install --package-lock-only` before bumping the npm package version.");
  process.exit(1);
}
