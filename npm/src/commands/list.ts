import { commandOnPath, execFileRunner, type Runner } from "../process.js";
import { cliBinaryName, DEFAULT_REGISTRY_URL, fetchRegistry, type Registry } from "../registry.js";

interface ListDeps {
  fetchRegistry: (url: string) => Promise<Registry>;
  commandOnPath: (binary: string) => Promise<string | null>;
  runner: Runner;
  stdout: (message: string) => void;
  stderr: (message: string) => void;
}

interface InstalledEntry {
  name: string;
  binary: string;
  version: string;
  description: string;
}

export function createListCommand(overrides: Partial<ListDeps> = {}) {
  const deps: ListDeps = {
    fetchRegistry: (url) => fetchRegistry(url),
    commandOnPath: (binary) => commandOnPath(binary),
    runner: execFileRunner,
    stdout: (message) => console.log(message),
    stderr: (message) => console.error(message),
    ...overrides,
  };

  return async function listCommandWithDeps(args: string[] = []): Promise<number> {
    const options = parseListArgs(args);
    if ("error" in options) {
      deps.stderr(options.error);
      return 1;
    }

    const registry = await deps.fetchRegistry(options.registryUrl);
    const installed: InstalledEntry[] = [];
    for (const entry of registry.entries) {
      const binary = cliBinaryName(entry);
      const binaryPath = await deps.commandOnPath(binary);
      if (!binaryPath) {
        continue;
      }
      const version = await binaryVersion(binary, deps.runner);
      installed.push({ name: entry.name, binary, version, description: entry.description });
    }

    if (options.json) {
      deps.stdout(JSON.stringify(installed, null, 2));
      return 0;
    }

    if (installed.length === 0) {
      deps.stdout("No Printing Press CLIs installed. Try `printing-press search <query>` or `printing-press install <name>`.");
      return 0;
    }

    deps.stdout(["Name", "Binary", "Version", "Description"].join("\t"));
    for (const entry of installed) {
      deps.stdout([entry.name, entry.binary, entry.version, entry.description].join("\t"));
    }
    return 0;
  };
}

export const listCommand = createListCommand();

function parseListArgs(args: string[]): { json: boolean; registryUrl: string } | { error: string } {
  const options = { json: false, registryUrl: DEFAULT_REGISTRY_URL };
  for (let i = 0; i < args.length; i++) {
    const arg = args[i]!;
    if (arg === "--json") {
      options.json = true;
    } else if (arg === "--registry-url") {
      const registryUrl = args[++i];
      if (!registryUrl) {
        return { error: "Missing value for --registry-url" };
      }
      options.registryUrl = registryUrl;
    } else {
      return { error: `Unknown list option: ${arg}` };
    }
  }
  return options;
}

async function binaryVersion(binary: string, runner: Runner): Promise<string> {
  const result = await runner(binary, ["--version"]);
  if (result.code !== 0) {
    return "unknown";
  }
  return result.stdout.trim().split(/\r?\n/)[0] || "unknown";
}
