import { mkdir, readFile, writeFile } from "node:fs/promises";
import { dirname, join } from "node:path";

/**
 * Filename of the installed-version stamp, parked alongside the `skills`
 * package's lockfile under `~/.agents/`. Keyed by CLI catalog name →
 * `printing_press_version` (the generator version that printed the installed
 * binary, sourced from the registry entry at install time).
 *
 * A parallel stamp (rather than reusing `~/.agents/.skill-lock.json`) keeps the
 * version tracking decoupled from the `skills` package's content-hash contract —
 * that file is owned by another tool whose format can change independently.
 */
export const STAMP_FILENAME = ".pp-cli-versions.json";

export interface VersionStampDeps {
  /** Home directory; when absent the stamp is skipped (read returns empty, write is a no-op). */
  home?: string;
  readFile: (path: string, encoding: BufferEncoding) => Promise<string>;
  writeFile: (path: string, data: string) => Promise<void>;
  mkdir: (path: string, options: { recursive: boolean }) => Promise<unknown>;
}

export const defaultStampDeps: VersionStampDeps = {
  home: process.env.HOME ?? process.env.USERPROFILE,
  readFile: (path, encoding) => readFile(path, encoding),
  writeFile: (path, data) => writeFile(path, data, "utf8"),
  mkdir: (path, options) => mkdir(path, options),
};

export function ppVersionsPath(home: string): string {
  return join(home, ".agents", STAMP_FILENAME);
}

/**
 * Read the installed-version stamp. Returns an empty map when the stamp is
 * missing, corrupt, or `home` is unknown — callers treat "no stamp" as
 * "version unknown → update" so the smart-update path degrades to the old
 * unconditional sweep for first-time runs and malformed stamps.
 */
export async function readInstalledVersions(
  deps: VersionStampDeps = defaultStampDeps,
): Promise<Record<string, string>> {
  if (!deps.home) {
    return {};
  }
  let raw: string;
  try {
    raw = await deps.readFile(ppVersionsPath(deps.home), "utf8");
  } catch {
    return {};
  }
  let parsed: unknown;
  try {
    parsed = JSON.parse(raw);
  } catch {
    return {};
  }
  return isStringRecord(parsed) ? { ...parsed } : {};
}

/**
 * Record (or clear) the installed `printing_press_version` for a single CLI.
 * A read-modify-write preserves sibling entries; concurrent installs in a
 * bundle run serially inside `installOne`, so there is no write-write race.
 * Failures are swallowed: a missing stamp must not fail an otherwise-successful
 * install, and the next install will retry the write.
 */
export async function writeInstalledVersion(
  name: string,
  version: string | undefined,
  deps: VersionStampDeps = defaultStampDeps,
): Promise<void> {
  if (!deps.home) {
    return;
  }
  const current = await readInstalledVersions(deps);
  if (version === undefined || version === "") {
    delete current[name];
  } else {
    current[name] = version;
  }
  const path = ppVersionsPath(deps.home);
  try {
    await deps.mkdir(dirname(path), { recursive: true });
    await deps.writeFile(path, JSON.stringify(current, null, 2) + "\n");
  } catch {
    // Stamp write is best-effort; a failed write just means the next
    // `update --stale-only` treats this CLI as unknown → re-installs.
  }
}

function isStringRecord(value: unknown): value is Record<string, string> {
  if (typeof value !== "object" || value === null || Array.isArray(value)) {
    return false;
  }
  for (const v of Object.values(value as Record<string, unknown>)) {
    if (typeof v !== "string") {
      return false;
    }
  }
  return true;
}
