import test from "node:test";
import assert from "node:assert/strict";
import {
  ppVersionsPath,
  readInstalledVersions,
  STAMP_FILENAME,
  writeInstalledVersion,
  type VersionStampDeps,
} from "../src/versions.js";

function inMemoryDeps(home: string): {
  deps: VersionStampDeps;
  files: Map<string, string>;
} {
  const files = new Map<string, string>();
  const deps: VersionStampDeps = {
    home,
    readFile: async (path) => files.get(path) ?? assert.fail(`read of unwritten file: ${path}`),
    writeFile: async (path, data) => {
      files.set(path, data);
    },
    mkdir: async () => {},
  };
  return { deps, files };
}

function lenientDeps(home: string): {
  deps: VersionStampDeps;
  files: Map<string, string>;
} {
  const files = new Map<string, string>();
  const deps: VersionStampDeps = {
    home,
    readFile: async (path) => {
      if (!files.has(path)) throw new Error("ENOENT");
      return files.get(path)!;
    },
    writeFile: async (path, data) => {
      files.set(path, data);
    },
    mkdir: async () => {},
  };
  return { deps, files };
}

test("ppVersionsPath lives under .agents next to the skills lockfile", () => {
  assert.equal(ppVersionsPath("/Users/example"), `/Users/example/.agents/${STAMP_FILENAME}`);
  assert.equal(STAMP_FILENAME, ".pp-cli-versions.json");
});

test("readInstalledVersions returns empty when the stamp is missing", async () => {
  const { deps } = lenientDeps("/Users/example");
  assert.deepEqual(await readInstalledVersions(deps), {});
});

test("readInstalledVersions returns empty when home is unset", async () => {
  const deps: VersionStampDeps = {
    home: undefined,
    readFile: async () => assert.fail("should not read"),
    writeFile: async () => assert.fail("should not write"),
    mkdir: async () => {},
  };
  assert.deepEqual(await readInstalledVersions(deps), {});
});

test("readInstalledVersions parses a valid stamp", async () => {
  const { deps, files } = lenientDeps("/Users/example");
  files.set(
    ppVersionsPath("/Users/example"),
    JSON.stringify({ espn: "4.24.0", "weather-goat": "4.23.0" }) + "\n",
  );
  assert.deepEqual(await readInstalledVersions(deps), {
    espn: "4.24.0",
    "weather-goat": "4.23.0",
  });
});

test("readInstalledVersions treats a corrupt stamp as empty", async () => {
  const { deps, files } = lenientDeps("/Users/example");
  files.set(ppVersionsPath("/Users/example"), "{not json");
  assert.deepEqual(await readInstalledVersions(deps), {});
});

test("readInstalledVersions rejects non-string values", async () => {
  const { deps, files } = lenientDeps("/Users/example");
  files.set(
    ppVersionsPath("/Users/example"),
    JSON.stringify({ espn: "4.24.0", bad: 123 }),
  );
  assert.deepEqual(await readInstalledVersions(deps), {});
});

test("writeInstalledVersion records the version for a single CLI", async () => {
  const { deps, files } = inMemoryDeps("/Users/example");
  await writeInstalledVersion("espn", "4.24.0", deps);
  const raw = files.get(ppVersionsPath("/Users/example"));
  assert.ok(raw);
  assert.deepEqual(JSON.parse(raw!), { espn: "4.24.0" });
});

test("writeInstalledVersion preserves sibling entries (read-modify-write)", async () => {
  const { deps, files } = inMemoryDeps("/Users/example");
  files.set(ppVersionsPath("/Users/example"), JSON.stringify({ espn: "4.24.0" }) + "\n");
  await writeInstalledVersion("linear", "4.25.0", deps);
  const raw = files.get(ppVersionsPath("/Users/example"));
  assert.deepEqual(JSON.parse(raw!), { espn: "4.24.0", linear: "4.25.0" });
});

test("writeInstalledVersion overwrites an existing entry in place", async () => {
  const { deps, files } = inMemoryDeps("/Users/example");
  files.set(ppVersionsPath("/Users/example"), JSON.stringify({ espn: "4.24.0" }) + "\n");
  await writeInstalledVersion("espn", "4.25.0", deps);
  const raw = files.get(ppVersionsPath("/Users/example"));
  assert.deepEqual(JSON.parse(raw!), { espn: "4.25.0" });
});

test("writeInstalledVersion with undefined clears the entry", async () => {
  const { deps, files } = inMemoryDeps("/Users/example");
  files.set(
    ppVersionsPath("/Users/example"),
    JSON.stringify({ espn: "4.24.0", linear: "4.25.0" }) + "\n",
  );
  await writeInstalledVersion("espn", undefined, deps);
  const raw = files.get(ppVersionsPath("/Users/example"));
  assert.deepEqual(JSON.parse(raw!), { linear: "4.25.0" });
});

test("writeInstalledVersion is a no-op when home is unset", async () => {
  let wrote = false;
  const deps: VersionStampDeps = {
    home: undefined,
    readFile: async () => "",
    writeFile: async () => {
      wrote = true;
    },
    mkdir: async () => {},
  };
  await writeInstalledVersion("espn", "4.24.0", deps);
  assert.equal(wrote, false);
});
