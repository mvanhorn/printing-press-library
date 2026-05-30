import { installCommand } from "./install.js";
import { commandOnPath } from "../process.js";
import { cliBinaryName, DEFAULT_REGISTRY_URL, fetchRegistry, type Registry } from "../registry.js";

interface UpdateDeps {
  fetchRegistry: (url: string) => Promise<Registry>;
  commandOnPath: (binary: string) => Promise<string | null>;
  install: (args: string[]) => Promise<number>;
  stdout: (message: string) => void;
  stderr: (message: string) => void;
}

export function createUpdateCommand(overrides: Partial<UpdateDeps> = {}) {
  const deps: UpdateDeps = {
    fetchRegistry: (url) => fetchRegistry(url),
    commandOnPath: (binary) => commandOnPath(binary),
    install: installCommand,
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
      return deps.install([parsed.name, ...parsed.installArgs]);
    }

    const registry = await deps.fetchRegistry(parsed.registryUrl);
    const installed = [];
    for (const entry of registry.entries) {
      if (await deps.commandOnPath(cliBinaryName(entry))) {
        installed.push(entry.name);
      }
    }

    if (installed.length === 0) {
      deps.stdout("No Printing Press CLIs found on PATH to refresh.");
      return 0;
    }

    let failures = 0;
    for (const name of installed) {
      const code = await deps.install([name, ...parsed.installArgs]);
      if (code !== 0) {
        failures++;
      }
    }
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
    } else if (arg === "--json" || arg === "--agent" || arg === "-a") {
      installArgs.push(arg);
      if (arg === "--agent" || arg === "-a") {
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
