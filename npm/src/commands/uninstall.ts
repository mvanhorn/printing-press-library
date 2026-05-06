import { rm } from "node:fs/promises";
import { commandOnPath, type RunResult } from "../process.js";
import { cliBinaryName, cliSkillName, DEFAULT_REGISTRY_URL, fetchRegistry, lookupByName, type Registry } from "../registry.js";
import { removeSkill } from "../skill.js";

interface UninstallDeps {
  fetchRegistry: (url: string) => Promise<Registry>;
  commandOnPath: (binary: string) => Promise<string | null>;
  removeFile: (path: string) => Promise<void>;
  removeSkill: (skillName: string, agents: string[]) => Promise<RunResult>;
  stdout: (message: string) => void;
  stderr: (message: string) => void;
}

export function createUninstallCommand(overrides: Partial<UninstallDeps> = {}) {
  const deps: UninstallDeps = {
    fetchRegistry: (url) => fetchRegistry(url),
    commandOnPath: (binary) => commandOnPath(binary),
    removeFile: (path) => rm(path, { force: true }),
    removeSkill: (skillName, agents) => removeSkill(skillName, { agents }),
    stdout: (message) => console.log(message),
    stderr: (message) => console.error(message),
    ...overrides,
  };

  return async function uninstallCommandWithDeps(args: string[]): Promise<number> {
    const parsed = parseUninstallArgs(args);
    if ("error" in parsed) {
      deps.stderr(parsed.error);
      deps.stderr("Usage: printing-press uninstall <name> --yes [--agent <agent>...]");
      return 1;
    }
    if (!parsed.yes) {
      deps.stderr("Refusing to uninstall without --yes.");
      return 1;
    }

    const registry = await deps.fetchRegistry(parsed.registryUrl);
    const entry = lookupByName(registry, parsed.name);
    if (!entry) {
      deps.stderr(`No Printing Press CLI found for "${parsed.name}".`);
      return 1;
    }

    const binary = cliBinaryName(entry);
    const binaryPath = await deps.commandOnPath(binary);
    if (binaryPath) {
      await deps.removeFile(binaryPath);
    }

    const skillName = cliSkillName(entry);
    const skill = await deps.removeSkill(skillName, parsed.agents);
    if (skill.code !== 0) {
      deps.stderr(`Failed to remove skill ${skillName}.`);
      if (skill.stderr.trim()) {
        deps.stderr(skill.stderr.trim());
      }
      return 1;
    }

    if (parsed.json) {
      deps.stdout(JSON.stringify({ ok: true, name: entry.name, binary, binaryPath, skill: skillName }, null, 2));
    } else {
      deps.stdout(`Uninstalled ${entry.name}`);
      if (binaryPath) {
        deps.stdout(`  removed binary: ${binaryPath}`);
      } else {
        deps.stdout(`  binary was not found on PATH: ${binary}`);
      }
      deps.stdout(`  removed skill: ${skillName}`);
    }
    return 0;
  };
}

export const uninstallCommand = createUninstallCommand();

function parseUninstallArgs(args: string[]):
  | { name: string; yes: boolean; json: boolean; agents: string[]; registryUrl: string }
  | { error: string } {
  let name: string | undefined;
  let yes = false;
  let json = false;
  let registryUrl = DEFAULT_REGISTRY_URL;
  const agents: string[] = [];

  for (let i = 0; i < args.length; i++) {
    const arg = args[i]!;
    if (arg === "--yes" || arg === "-y") {
      yes = true;
    } else if (arg === "--json") {
      json = true;
    } else if (arg === "--agent" || arg === "-a") {
      const value = args[++i];
      if (!value) return { error: `Missing value for ${arg}` };
      agents.push(value);
    } else if (arg === "--registry-url") {
      const value = args[++i];
      if (!value) return { error: "Missing value for --registry-url" };
      registryUrl = value;
    } else if (arg.startsWith("-")) {
      return { error: `Unknown uninstall option: ${arg}` };
    } else if (!name) {
      name = arg;
    } else {
      return { error: `Unexpected argument: ${arg}` };
    }
  }

  return name ? { name, yes, json, agents, registryUrl } : { error: "Missing CLI name" };
}
