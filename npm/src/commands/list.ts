import { commandOnPath, execFileRunner, type Runner } from "../process.js";
import {
  cliBinaryName,
  DEFAULT_REGISTRY_URL,
  fetchRegistry,
  type Registry,
  type RegistryEntry,
} from "../registry.js";
import { renderCatalogEntries, renderInstalledEntries } from "../format.js";
import { NPX_COMMAND_PREFIX } from "../constants.js";

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
    if (!options.installed) {
      const entries = filterCatalogEntries(registry.entries, options.category);
      if (options.json) {
        deps.stdout(JSON.stringify(entries, null, 2));
        return 0;
      }

      if (entries.length === 0) {
        const suffix = options.category ? ` in category "${options.category}"` : "";
        deps.stdout(`No Printing Press CLIs found${suffix}.`);
        return 0;
      }

      for (const line of renderCatalogEntries(entries)) {
        deps.stdout(line);
      }
      return 0;
    }

    const installed: InstalledEntry[] = [];
    for (const entry of filterCatalogEntries(registry.entries, options.category)) {
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
      const suffix = options.category ? ` in category "${options.category}"` : "";
      deps.stdout(`No Printing Press CLIs installed${suffix}. Try \`${NPX_COMMAND_PREFIX} search <query>\` or \`${NPX_COMMAND_PREFIX} install <name>\`.`);
      return 0;
    }

    for (const line of renderInstalledEntries(installed)) {
      deps.stdout(line);
    }
    return 0;
  };
}

export const listCommand = createListCommand();

function parseListArgs(args: string[]):
  | { json: boolean; installed: boolean; category?: string; registryUrl: string }
  | { error: string } {
  const options: { json: boolean; installed: boolean; category?: string; registryUrl: string } = {
    json: false,
    installed: false,
    registryUrl: DEFAULT_REGISTRY_URL,
  };
  for (let i = 0; i < args.length; i++) {
    const arg = args[i]!;
    if (arg === "--json") {
      options.json = true;
    } else if (arg === "--installed") {
      options.installed = true;
    } else if (arg === "--category") {
      const category = args[++i];
      if (!category) {
        return { error: "Missing value for --category" };
      }
      options.category = category;
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

function filterCatalogEntries(entries: RegistryEntry[], category?: string): RegistryEntry[] {
  const filtered = category
    ? entries.filter((entry) => entry.category.toLowerCase() === category.toLowerCase())
    : entries;
  return [...filtered].sort((a, b) => a.category.localeCompare(b.category) || a.name.localeCompare(b.name));
}

async function binaryVersion(binary: string, runner: Runner): Promise<string> {
  const result = await runner(binary, ["--version"]);
  if (result.code !== 0) {
    return "unknown";
  }
  return result.stdout.trim().split(/\r?\n/)[0] || "unknown";
}
