import { createInstallCommand } from "./install.js";
import { mapWithConcurrency } from "../concurrency.js";
import { commandOnPath } from "../process.js";
import { cliBinaryName, DEFAULT_REGISTRY_URL, fetchRegistry, type Registry } from "../registry.js";
import { defaultStampDeps, readInstalledVersions } from "../versions.js";

/** Output sinks handed to a single buffered install run. */
interface InstallIO {
  stdout: (message: string) => void;
  stderr: (message: string) => void;
}

/** One captured output line, tagged with its stream so emission order survives buffering. */
interface BufferedLine {
  stream: "out" | "err";
  message: string;
}

interface UpdateDeps {
  fetchRegistry: (url: string) => Promise<Registry>;
  commandOnPath: (binary: string) => Promise<string | null>;
  /**
   * Read the installed-version stamp (`~/.agents/.pp-cli-versions.json`).
   * Injectable so update tests can simulate installed-at-version-X scenarios
   * without touching the real filesystem.
   */
  readInstalledVersions: () => Promise<Record<string, string>>;
  /**
   * Build an install command bound to the given output sinks. A factory (rather
   * than a single shared install fn) lets the bulk path give each concurrent run
   * its own buffer, so parallel installs don't interleave their lines.
   */
  createInstall: (io: InstallIO) => (args: string[]) => Promise<number>;
  stdout: (message: string) => void;
  stderr: (message: string) => void;
}

// `which`/`where` probes are cheap but the detection sweep covers the whole
// catalog (hundreds of entries), so cap the fan-out to avoid a process storm.
const DETECT_CONCURRENCY = 16;
// Installs are network-bound (go-proxy `@latest` resolution + skill fetch), the
// dominant cost in a bulk update. Run several at once, but cap to share the
// proxy politely and bound concurrent global skill writes.
const INSTALL_CONCURRENCY = 6;

export function createUpdateCommand(overrides: Partial<UpdateDeps> = {}) {
  const deps: UpdateDeps = {
    fetchRegistry: (url) => fetchRegistry(url),
    commandOnPath: (binary) => commandOnPath(binary),
    readInstalledVersions: () => readInstalledVersions(defaultStampDeps),
    createInstall: (io) => createInstallCommand({ stdout: io.stdout, stderr: io.stderr }),
    stdout: (message) => console.log(message),
    stderr: (message) => console.error(message),
    ...overrides,
  };

  return async function updateCommandWithDeps(args: string[]): Promise<number> {
    const parsed = parseUpdateArgs(args);
    if ("error" in parsed) {
      deps.stderr(parsed.error);
      return 1;
    }

    if (parsed.name) {
      // Single target: stream output straight through, no buffering needed.
      // The stale-only flag is intentionally ignored here — naming a CLI
      // explicitly is a force-refresh of that one CLI.
      const install = deps.createInstall({ stdout: deps.stdout, stderr: deps.stderr });
      return install([parsed.name, ...parsed.installArgs]);
    }

    const registry = await deps.fetchRegistry(parsed.registryUrl);
    const detected = await mapWithConcurrency(registry.entries, DETECT_CONCURRENCY, async (entry) => {
      try {
        return (await deps.commandOnPath(cliBinaryName(entry))) ? entry.name : null;
      } catch {
        // A failed PATH probe (rare `which`/`where` spawn error) shouldn't abort
        // the whole update — treat the entry as not installed and move on.
        return null;
      }
    });
    const installed = detected.filter((name): name is string => name !== null);

    if (installed.length === 0) {
      deps.stdout("No Printing Press CLIs found on PATH to refresh.");
      return 0;
    }

    // Partition into "current" (skip) and "stale/unknown" (re-install) when
    // running in stale-only mode (the default). A CLI is current only when both
    // the stamp and the registry carry a `printing_press_version` and they match;
    // any uncertainty (missing stamp, missing field, mismatch) → re-install.
    const entryByName = new Map(registry.entries.map((e) => [e.name, e]));
    const json = parsed.installArgs.includes("--json");

    let skipped: Array<{ name: string; version: string }> = [];
    let toUpdate: string[];

    if (parsed.staleOnly) {
      const stamp = await deps.readInstalledVersions();
      skipped = [];
      toUpdate = [];
      for (const name of installed) {
        const registryVersion = entryByName.get(name)?.printing_press_version;
        const installedVersion = stamp[name];
        if (registryVersion && installedVersion && registryVersion === installedVersion) {
          skipped.push({ name, version: registryVersion });
        } else {
          toUpdate.push(name);
        }
      }
    } else {
      toUpdate = installed;
    }

    // Emit skip notices first (immediate, before the buffered install output).
    for (const { name, version } of skipped) {
      if (json) {
        deps.stdout(JSON.stringify({ ok: true, name, skipped: true, printing_press_version: version }));
      } else {
        deps.stdout(`${name} is current (printing_press_version ${version}), skipping.`);
      }
    }

    if (toUpdate.length === 0) {
      if (!json) {
        const word = skipped.length === 1 ? "CLI is" : "CLIs are";
        deps.stdout(`All ${skipped.length} detected ${word} current. Re-run with --all to force re-install.`);
      }
      return 0;
    }

    // Refresh concurrently, but record each run's output in emission order and
    // replay it per CLI in catalog order — so parallel runs don't interleave into
    // scrambled lines, while stdout/stderr ordering within a CLI is preserved.
    const results = await mapWithConcurrency(toUpdate, INSTALL_CONCURRENCY, async (name) => {
      const lines: BufferedLine[] = [];
      const install = deps.createInstall({
        stdout: (message) => lines.push({ stream: "out", message }),
        stderr: (message) => lines.push({ stream: "err", message }),
      });
      let code: number;
      try {
        code = await install([name, ...parsed.installArgs]);
      } catch (error) {
        // install resolves with an exit code rather than throwing; guard anyway
        // so one unexpected throw can't reject the whole concurrent batch.
        lines.push({ stream: "err", message: error instanceof Error ? error.message : String(error) });
        code = 1;
      }
      return { code, lines };
    });

    for (const { lines } of results) {
      for (const { stream, message } of lines) {
        (stream === "out" ? deps.stdout : deps.stderr)(message);
      }
    }

    if (!json && skipped.length > 0) {
      const updated = results.filter((r) => r.code === 0).length;
      const failed = toUpdate.length - updated;
      deps.stdout("");
      if (failed === 0) {
        deps.stdout(
          `Updated ${updated} of ${toUpdate.length}; ${skipped.length} current. Re-run with --all to force re-install.`,
        );
      } else {
        deps.stdout(
          `Updated ${updated} of ${toUpdate.length}; ${skipped.length} current, ${failed} failed. Re-run with --all to force re-install.`,
        );
      }
    }

    const failures = results.filter((result) => result.code !== 0).length;
    return failures === 0 ? 0 : 1;
  };
}

export const updateCommand = createUpdateCommand();

function parseUpdateArgs(args: string[]):
  | { name?: string; registryUrl: string; installArgs: string[]; staleOnly: boolean }
  | { error: string } {
  let name: string | undefined;
  let registryUrl = DEFAULT_REGISTRY_URL;
  // `--stale-only` is the default: only re-install CLIs whose stamped
  // `printing_press_version` is missing or behind the registry. `--all` opts
  // back into the unconditional sweep (the pre-smart-update behavior).
  let staleOnly = true;
  const installArgs: string[] = [];

  for (let i = 0; i < args.length; i++) {
    const arg = args[i]!;
    if (arg === "--stale-only") {
      staleOnly = true;
    } else if (arg === "--all") {
      staleOnly = false;
    } else if (arg === "--registry-url") {
      const value = args[++i];
      if (!value) {
        return { error: "Missing value for --registry-url" };
      }
      registryUrl = value;
      installArgs.push("--registry-url", value);
    } else if (arg === "--json" || arg === "--agent" || arg === "-a" || arg === "--bin-dir") {
      installArgs.push(arg);
      if (arg === "--agent" || arg === "-a" || arg === "--bin-dir") {
        const value = args[++i];
        if (!value) {
          return { error: `Missing value for ${arg}` };
        }
        installArgs.push(value);
      }
    } else if (arg.startsWith("-")) {
      return { error: `Unknown option: ${arg}` };
    } else if (!name) {
      name = arg;
    } else {
      return { error: `Unexpected argument: ${arg}` };
    }
  }

  return { name, registryUrl, installArgs, staleOnly };
}
