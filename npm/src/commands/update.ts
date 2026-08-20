import { createInstallCommand } from "./install.js";
import { mapWithConcurrency } from "../concurrency.js";
import { commandOnPath } from "../process.js";
import { cliBinaryName, DEFAULT_REGISTRY_URL, fetchRegistry, type Registry } from "../registry.js";

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

    // Refresh concurrently, but record each run's output in emission order and
    // replay it per CLI in catalog order — so parallel runs don't interleave into
    // scrambled lines, while stdout/stderr ordering within a CLI is preserved.
    const results = await mapWithConcurrency(installed, INSTALL_CONCURRENCY, async (name) => {
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

    const failures = results.filter((result) => result.code !== 0).length;
    return failures === 0 ? 0 : 1;
  };
}

export const updateCommand = createUpdateCommand();

function parseUpdateArgs(args: string[]):
  | { name?: string; registryUrl: string; installArgs: string[] }
  | { error: string } {
  let name: string | undefined;
  let registryUrl = DEFAULT_REGISTRY_URL;
  const installArgs: string[] = [];

  for (let i = 0; i < args.length; i++) {
    const arg = args[i]!;
    if (arg === "--registry-url") {
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

  return { name, registryUrl, installArgs };
}
