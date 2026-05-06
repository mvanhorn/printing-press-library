import { BUNDLES, isBundle } from "../bundles.js";
import { detectGo, goInstall, type GoDetection } from "../go.js";
import { commandOnPath, type RunResult } from "../process.js";
import {
  cliBinaryName,
  cliSkillName,
  DEFAULT_REGISTRY_URL,
  fetchGoModulePath,
  fetchRegistry,
  lookupByName,
  type Registry,
} from "../registry.js";
import { installSkill } from "../skill.js";

interface InstallOptions {
  agents: string[];
  json: boolean;
  registryUrl: string;
  cliOnly: boolean;
  skillOnly: boolean;
}

interface InstallDeps {
  fetchRegistry: (url: string) => Promise<Registry>;
  resolveModulePath: (entryPath: string, registryUrl: string) => Promise<string | null>;
  detectGo: () => Promise<GoDetection>;
  goInstall: (modulePath: string, ref: string, env?: NodeJS.ProcessEnv) => Promise<RunResult>;
  commandOnPath: (binary: string) => Promise<string | null>;
  installSkill: (skillName: string, agents: string[]) => Promise<RunResult>;
  stdout: (message: string) => void;
  stderr: (message: string) => void;
  platform: NodeJS.Platform;
}

interface InstallSummary {
  name: string;
  binary?: string;
  modulePath?: string;
  skill?: string;
  binaryPath?: string;
  authEnvVars: string[];
}

interface InstallOutcome {
  ok: boolean;
  name: string;
  data?: InstallSummary;
  error?: string;
}

export function createInstallCommand(overrides: Partial<InstallDeps> = {}) {
  const deps: InstallDeps = {
    fetchRegistry: (url) => fetchRegistry(url),
    resolveModulePath: (entryPath, registryUrl) => fetchGoModulePath(entryPath, registryUrl),
    detectGo: () => detectGo(),
    goInstall: (modulePath, ref, env) => goInstall(modulePath, { ref, env }),
    commandOnPath: (binary) => commandOnPath(binary),
    installSkill: (skillName, agents) => installSkill(skillName, { agents }),
    stdout: (message) => console.log(message),
    stderr: (message) => console.error(message),
    platform: process.platform,
    ...overrides,
  };

  return async function installCommandWithDeps(args: string[]): Promise<number> {
    const parsed = parseInstallArgs(args);
    if ("error" in parsed) {
      deps.stderr(parsed.error);
      deps.stderr("Usage: printing-press install <name|bundle>... [--agent <agent>...] [--json]");
      return 1;
    }

    const expanded = expandBundles(parsed.names, parsed.options, deps);

    let registry: Registry;
    try {
      registry = await deps.fetchRegistry(parsed.options.registryUrl);
    } catch (error) {
      deps.stderr(error instanceof Error ? error.message : String(error));
      return 1;
    }

    const outcomes: InstallOutcome[] = [];
    for (const name of expanded) {
      const outcome = await installOne(name, registry, parsed.options, deps);
      outcomes.push(outcome);
      if (outcome.error === "go missing") {
        // Go is a global precondition; no point retrying it for the rest.
        break;
      }
    }

    return reportResults(outcomes, parsed.options, deps);
  };
}

async function installOne(
  name: string,
  registry: Registry,
  options: InstallOptions,
  deps: InstallDeps,
): Promise<InstallOutcome> {
  const entry = lookupByName(registry, name);
  if (!entry) {
    deps.stderr(`No Printing Press CLI found for "${name}". Try \`printing-press search ${name}\`.`);
    return { ok: false, name, error: "not in catalog" };
  }

  const summary: InstallSummary = {
    name: entry.name,
    authEnvVars: entry.mcp?.env_vars ?? [],
  };

  if (!options.skillOnly) {
    const go = await deps.detectGo();
    if (!go.installed) {
      deps.stderr(goMissingMessage(deps.platform));
      return { ok: false, name: entry.name, error: "go missing" };
    }

    const binary = cliBinaryName(entry);
    const moduleRoot =
      (await deps.resolveModulePath(entry.path, options.registryUrl)) ??
      `github.com/mvanhorn/printing-press-library/${entry.path}`;
    const modulePath = `${moduleRoot}/cmd/${binary}`;

    const install = await installGoWithFallback(deps, modulePath);
    if (install.code !== 0) {
      deps.stderr(`go install failed for ${modulePath}`);
      if (install.stderr.trim()) {
        deps.stderr(install.stderr.trim());
      }
      return { ok: false, name: entry.name, error: "go install failed" };
    }

    const binaryPath = await deps.commandOnPath(binary);
    if (!binaryPath) {
      deps.stderr(pathMessage(binary));
      return { ok: false, name: entry.name, error: "binary not on PATH" };
    }

    summary.binary = binary;
    summary.modulePath = modulePath;
    summary.binaryPath = binaryPath;
  }

  if (!options.cliOnly) {
    const skillName = cliSkillName(entry);
    const skill = await deps.installSkill(skillName, options.agents);
    if (skill.code !== 0) {
      const binaryNote = summary.binaryPath
        ? ` The binary remains installed at ${summary.binaryPath}.`
        : "";
      deps.stderr(`Skill install failed for ${skillName}.${binaryNote}`);
      if (skill.stderr.trim()) {
        deps.stderr(skill.stderr.trim());
      }
      return { ok: false, name: entry.name, error: "skill install failed" };
    }
    summary.skill = skillName;
  }

  if (!options.json) {
    deps.stdout(`Installed ${entry.name}`);
    if (summary.binaryPath) {
      deps.stdout(`  binary: ${summary.binaryPath}`);
    }
    if (summary.skill) {
      deps.stdout(`  skill: ${summary.skill}`);
    }
    if (summary.authEnvVars.length > 0) {
      deps.stdout(`  auth env vars: ${summary.authEnvVars.join(", ")}`);
    }
  }

  return { ok: true, name: entry.name, data: summary };
}

function expandBundles(names: string[], options: InstallOptions, deps: InstallDeps): string[] {
  const expanded: string[] = [];
  for (const name of names) {
    if (isBundle(name)) {
      const members = BUNDLES[name]!;
      if (!options.json) {
        deps.stdout(`Bundle "${name}" → ${members.join(", ")}`);
      }
      expanded.push(...members);
    } else {
      expanded.push(name);
    }
  }
  return expanded;
}

function reportResults(outcomes: InstallOutcome[], options: InstallOptions, deps: InstallDeps): number {
  const failures = outcomes.filter((o) => !o.ok);

  if (options.json) {
    // Backward-compatible flat shape for the single-success case.
    if (outcomes.length === 1 && outcomes[0]!.ok) {
      deps.stdout(JSON.stringify({ ok: true, ...outcomes[0]!.data }, null, 2));
      return 0;
    }
    deps.stdout(
      JSON.stringify(
        {
          ok: failures.length === 0,
          results: outcomes.map((o) => ({
            ok: o.ok,
            name: o.name,
            ...(o.data ?? {}),
            ...(o.error ? { error: o.error } : {}),
          })),
        },
        null,
        2,
      ),
    );
    return failures.length === 0 ? 0 : 1;
  }

  if (outcomes.length > 1) {
    deps.stdout("");
    if (failures.length === 0) {
      deps.stdout(`Installed ${outcomes.length} CLI(s).`);
    } else {
      const ok = outcomes.length - failures.length;
      const failedNames = failures.map((f) => f.name).join(", ");
      deps.stdout(`Installed ${ok} of ${outcomes.length}; failed: ${failedNames}.`);
    }
  }

  return failures.length === 0 ? 0 : 1;
}

export const installCommand = createInstallCommand();

function parseInstallArgs(args: string[]):
  | { names: string[]; options: InstallOptions }
  | { error: string } {
  const options: InstallOptions = {
    agents: [],
    json: false,
    registryUrl: DEFAULT_REGISTRY_URL,
    cliOnly: false,
    skillOnly: false,
  };
  const names: string[] = [];

  for (let i = 0; i < args.length; i++) {
    const arg = args[i]!;
    if (arg === "--json") {
      options.json = true;
    } else if (arg === "--cli-only") {
      options.cliOnly = true;
    } else if (arg === "--skill-only") {
      options.skillOnly = true;
    } else if (arg === "--agent" || arg === "-a") {
      const agent = args[++i];
      if (!agent) {
        return { error: "Missing value for --agent" };
      }
      options.agents.push(agent);
    } else if (arg === "--registry-url") {
      const registryUrl = args[++i];
      if (!registryUrl) {
        return { error: "Missing value for --registry-url" };
      }
      options.registryUrl = registryUrl;
    } else if (arg.startsWith("-")) {
      return { error: `Unknown install option: ${arg}` };
    } else {
      names.push(arg);
    }
  }

  if (options.cliOnly && options.skillOnly) {
    return { error: "--cli-only and --skill-only are mutually exclusive" };
  }

  if (names.length === 0) {
    return { error: "Missing CLI name or bundle" };
  }

  return { names, options };
}

async function installGoWithFallback(deps: InstallDeps, modulePath: string): Promise<RunResult> {
  const latest = await deps.goInstall(modulePath, "latest");
  if (latest.code === 0) {
    return latest;
  }

  const env = {
    GOPRIVATE: "github.com/mvanhorn/*",
    GOFLAGS: "-mod=mod",
  };
  const main = await deps.goInstall(modulePath, "main", env);
  return main.code === 0 ? main : combineFailures(latest, main);
}

function combineFailures(latest: RunResult, main: RunResult): RunResult {
  return {
    code: main.code || latest.code || 1,
    stdout: [latest.stdout, main.stdout].filter(Boolean).join("\n"),
    stderr: [
      "go install @latest failed:",
      latest.stderr.trim(),
      "go install @main fallback failed:",
      main.stderr.trim(),
    ]
      .filter(Boolean)
      .join("\n"),
  };
}

function goMissingMessage(platform: NodeJS.Platform): string {
  const installHint =
    platform === "darwin"
      ? "Install Go with: brew install go"
      : platform === "win32"
        ? "Install Go with: winget install GoLang.Go"
        : "Install Go from your package manager or https://go.dev/dl/";
  return `Go is required to install Printing Press CLIs. ${installHint}`;
}

function pathMessage(binary: string): string {
  return `${binary} was installed, but it is not on PATH. Add $(go env GOPATH)/bin, usually $HOME/go/bin, to PATH and retry.`;
}
