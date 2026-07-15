import test from "node:test";
import assert from "node:assert/strict";
import { createInstallCommand } from "../src/commands/install.js";
import type { GoDetection, GoInstallDir } from "../src/go.js";
import type { RunResult } from "../src/process.js";
import type { Registry } from "../src/registry.js";

function goBinDir(binDir: string): GoInstallDir {
  return { binDir, gobin: binDir, gopath: "/Users/example/go" };
}

const registry: Registry = {
  schema_version: 2,
  entries: [
    {
      name: "espn",
      category: "sports",
      api: "ESPN",
      description: "Sports scores",
      path: "library/sports/espn",
      mcp: {
        binary: "espn-pp-mcp",
        transports: ["stdio"],
        tool_count: 10,
        auth_type: "none",
        env_vars: [],
      },
    },
  ],
};

function ok(stdout = ""): RunResult {
  return { code: 0, stdout, stderr: "" };
}

function fail(stderr: string): RunResult {
  return { code: 1, stdout: "", stderr };
}

test("install command installs binary and skill", async () => {
  const goCalls: Array<{ modulePath: string; ref: string; env?: NodeJS.ProcessEnv }> = [];
  const skillCalls: Array<{ skillName: string; agents: string[] }> = [];
  const stdout: string[] = [];

  const command = createInstallCommand({
    fetchRegistry: async () => registry,
    resolveModulePath: async () =>
      "github.com/mvanhorn/printing-press-library/library/sports/espn",
    detectGo: async () => ({ installed: true, version: "1.23.4" }),
    goInstall: async (modulePath, ref, env) => {
      goCalls.push({ modulePath, ref, env });
      return ok();
    },
    goInstallDir: async () => goBinDir("/Users/example/go/bin"),
    commandOnPath: async () => "/Users/example/.local/bin/espn-pp-cli",
    home: "/Users/example",
    env: {},
    mkdir: async () => {},
    installSkill: async (skillName, agents) => {
      skillCalls.push({ skillName, agents });
      return ok();
    },
    stdout: (message) => stdout.push(message),
    stderr: () => {},
  });

  assert.equal(await command(["espn", "--agent", "claude-code"]), 0);

  assert.equal(goCalls.length, 1);
  assert.equal(
    goCalls[0]!.modulePath,
    "github.com/mvanhorn/printing-press-library/library/sports/espn/cmd/espn-pp-cli",
  );
  assert.equal(goCalls[0]!.ref, "latest");
  assert.equal(goCalls[0]!.env?.GOBIN, "/Users/example/.local/bin");
  assert.deepEqual(skillCalls, [{ skillName: "pp-espn", agents: ["claude-code"] }]);
  assert.match(stdout.join("\n"), /Installed espn/);
});

test("install command reports unknown CLIs", async () => {
  const stderr: string[] = [];
  const command = createInstallCommand({
    fetchRegistry: async () => registry,
    resolveModulePath: async () => null,
    stderr: (message) => stderr.push(message),
  });

  assert.equal(await command(["missing"]), 1);
  assert.match(stderr.join("\n"), /No Printing Press CLI found/);
});

test("install command stops when Go is missing", async () => {
  const calls: string[] = [];
  const stderr: string[] = [];
  const command = createInstallCommand({
    fetchRegistry: async () => registry,
    resolveModulePath: async () => null,
    detectGo: async (): Promise<GoDetection> => ({ installed: false }),
    goInstall: async () => {
      calls.push("goInstall");
      return ok();
    },
    stderr: (message) => stderr.push(message),
    platform: "darwin",
  });

  assert.equal(await command(["espn"]), 1);
  assert.deepEqual(calls, []);
  assert.match(stderr.join("\n"), /brew install go/);
});

test("install command surfaces go install failure when @latest fails", async () => {
  const refs: string[] = [];
  const stderr: string[] = [];
  const command = createInstallCommand({
    fetchRegistry: async () => registry,
    resolveModulePath: async () => null,
    detectGo: async () => ({ installed: true }),
    goInstall: async (_modulePath, ref) => {
      refs.push(ref);
      return fail("proxy miss");
    },
    goInstallDir: async () => goBinDir("/Users/example/go/bin"),
    commandOnPath: async () => "/Users/example/go/bin/espn-pp-cli",
    installSkill: async () => ok(),
    stdout: () => {},
    stderr: (message) => stderr.push(message),
  });

  assert.equal(await command(["espn"]), 1);
  assert.deepEqual(refs, ["latest"]);
  assert.match(stderr.join("\n"), /go install failed/);
});

test("install command warns but still installs skill when binary is not on PATH", async () => {
  const skillCalls: string[] = [];
  const stderr: string[] = [];
  const stdout: string[] = [];
  const command = createInstallCommand({
    fetchRegistry: async () => registry,
    resolveModulePath: async () => null,
    detectGo: async () => ({ installed: true }),
    goInstall: async () => ok(),
    goInstallDir: async () => goBinDir("/Users/example/go/bin"),
    commandOnPath: async () => null,
    installSkill: async () => {
      skillCalls.push("skill");
      return ok();
    },
    stdout: (message) => stdout.push(message),
    stderr: (message) => stderr.push(message),
  });

  assert.equal(await command(["espn"]), 0);
  assert.deepEqual(skillCalls, ["skill"]);
  assert.match(stderr.join("\n"), /not on PATH/);
  assert.match(stdout.join("\n"), /warning: binary is not on PATH/);
});

test("not-on-PATH warning is zsh-flavored on macOS", async () => {
  const stderr: string[] = [];
  const command = createInstallCommand({
    fetchRegistry: async () => registry,
    resolveModulePath: async () => null,
    detectGo: async () => ({ installed: true }),
    goInstall: async () => ok(),
    goInstallDir: async () => goBinDir("/Users/amosclaw/go/bin"),
    commandOnPath: async () => null,
    mkdir: async () => {},
    installSkill: async () => ok(),
    stdout: () => {},
    stderr: (message) => stderr.push(message),
    platform: "darwin",
    shell: "/bin/zsh",
    home: "/Users/amosclaw",
  });

  assert.equal(await command(["espn"]), 0);
  const out = stderr.join("\n");
  assert.match(out, /installed espn-pp-cli at \/Users\/amosclaw\/\.local\/bin\/espn-pp-cli/);
  assert.match(out, /~\/\.zshrc/);
  assert.match(out, /export PATH="\$HOME\/\.local\/bin:\$PATH"/);
});

test("not-on-PATH warning is PowerShell-flavored on Windows", async () => {
  const stderr: string[] = [];
  const command = createInstallCommand({
    fetchRegistry: async () => registry,
    resolveModulePath: async () => null,
    detectGo: async () => ({ installed: true }),
    goInstall: async () => ok(),
    goInstallDir: async () => goBinDir("C:\\Users\\you\\go\\bin"),
    commandOnPath: async () => null,
    mkdir: async () => {},
    installSkill: async () => ok(),
    stdout: () => {},
    stderr: (message) => stderr.push(message),
    platform: "win32",
    shell: undefined,
    home: "C:\\Users\\you",
  });

  assert.equal(await command(["espn"]), 0);
  const out = stderr.join("\n");
  assert.match(out, /SetEnvironmentVariable/);
  assert.doesNotMatch(out, /setx/);
});

test("install command reports skill install failure without hiding binary", async () => {
  const stderr: string[] = [];
  const command = createInstallCommand({
    fetchRegistry: async () => registry,
    resolveModulePath: async () => null,
    detectGo: async () => ({ installed: true }),
    goInstall: async () => ok(),
    goInstallDir: async () => goBinDir("/Users/example/go/bin"),
    commandOnPath: async () => "/Users/example/go/bin/espn-pp-cli",
    mkdir: async () => {},
    installSkill: async () => fail("network down"),
    stderr: (message) => stderr.push(message),
    home: "/Users/example",
    env: {},
  });

  assert.equal(await command(["espn"]), 1);
  assert.match(stderr.join("\n"), /binary remains installed/);
  assert.match(stderr.join("\n"), /network down/);
});

test("install command emits JSON when requested", async () => {
  const stdout: string[] = [];
  const command = createInstallCommand({
    fetchRegistry: async () => registry,
    resolveModulePath: async () => null,
    detectGo: async () => ({ installed: true }),
    goInstall: async () => ok(),
    goInstallDir: async () => goBinDir("/Users/example/go/bin"),
    commandOnPath: async () => "/Users/example/go/bin/espn-pp-cli",
    mkdir: async () => {},
    installSkill: async () => ok(),
    stdout: (message) => stdout.push(message),
    stderr: () => {},
    home: "/Users/example",
    env: {},
  });

  assert.equal(await command(["espn", "--json"]), 0);
  assert.equal(JSON.parse(stdout[0]!).skill, "pp-espn");
});

test("install command installs multiple CLIs in one call", async () => {
  const multiRegistry: Registry = {
    schema_version: 1,
    entries: [
      {
        name: "espn",
        category: "sports",
        api: "ESPN",
        description: "Sports scores",
        path: "library/sports/espn",
      },
      {
        name: "linear",
        category: "project-management",
        api: "Linear",
        description: "Issues",
        path: "library/project-management/linear",
      },
    ],
  };
  const installed: string[] = [];
  const skills: string[] = [];
  const stdout: string[] = [];
  const command = createInstallCommand({
    fetchRegistry: async () => multiRegistry,
    resolveModulePath: async () => null,
    detectGo: async () => ({ installed: true }),
    goInstall: async (modulePath) => {
      installed.push(modulePath);
      return ok();
    },
    goInstallDir: async () => goBinDir("/Users/example/go/bin"),
    commandOnPath: async (binary) => `/Users/example/go/bin/${binary}`,
    mkdir: async () => {},
    installSkill: async (skillName) => {
      skills.push(skillName);
      return ok();
    },
    stdout: (message) => stdout.push(message),
    stderr: () => {},
    home: "/Users/example",
    env: {},
  });

  assert.equal(await command(["espn", "linear"]), 0);
  assert.equal(installed.length, 2);
  assert.deepEqual(new Set(skills), new Set(["pp-espn", "pp-linear"]));
  assert.match(stdout.join("\n"), /Installed 2 CLI/);
});

test("install command expands the starter-pack bundle", async () => {
  const bundleRegistry: Registry = {
    schema_version: 1,
    entries: [
      { name: "espn", category: "media", api: "ESPN", description: "x", path: "library/media/espn" },
      { name: "flight-goat", category: "travel", api: "FlightGoat", description: "x", path: "library/travel/flightgoat" },
      { name: "movie-goat", category: "media", api: "MovieGoat", description: "x", path: "library/media/movie-goat" },
      { name: "recipe-goat", category: "food", api: "RecipeGoat", description: "x", path: "library/food/recipe-goat" },
    ],
  };
  const installed: string[] = [];
  const stdout: string[] = [];
  const command = createInstallCommand({
    fetchRegistry: async () => bundleRegistry,
    resolveModulePath: async () => null,
    detectGo: async () => ({ installed: true }),
    goInstall: async (modulePath) => {
      installed.push(modulePath);
      return ok();
    },
    goInstallDir: async () => goBinDir("/Users/example/go/bin"),
    commandOnPath: async (binary) => `/Users/example/go/bin/${binary}`,
    mkdir: async () => {},
    installSkill: async () => ok(),
    stdout: (message) => stdout.push(message),
    stderr: () => {},
    home: "/Users/example",
    env: {},
  });

  assert.equal(await command(["starter-pack"]), 0);
  assert.equal(installed.length, 4);
  assert.match(stdout.join("\n"), /Bundle "starter-pack"/);
  assert.match(stdout.join("\n"), /Installed 4 CLI/);
});

test("install command continues after a partial multi-name failure", async () => {
  const partialRegistry: Registry = {
    schema_version: 1,
    entries: [
      { name: "espn", category: "sports", api: "ESPN", description: "x", path: "library/sports/espn" },
    ],
  };
  const stdout: string[] = [];
  const command = createInstallCommand({
    fetchRegistry: async () => partialRegistry,
    resolveModulePath: async () => null,
    detectGo: async () => ({ installed: true }),
    goInstall: async () => ok(),
    goInstallDir: async () => goBinDir("/Users/example/go/bin"),
    commandOnPath: async (binary) => `/Users/example/go/bin/${binary}`,
    mkdir: async () => {},
    installSkill: async () => ok(),
    stdout: (message) => stdout.push(message),
    stderr: () => {},
    home: "/Users/example",
    env: {},
  });

  assert.equal(await command(["espn", "made-up-name"]), 1);
  assert.match(stdout.join("\n"), /Installed 1 of 2; failed: made-up-name/);
});

test("install command with --cli-only skips skill install", async () => {
  const goCalls: string[] = [];
  const skillCalls: string[] = [];
  const stdout: string[] = [];
  const command = createInstallCommand({
    fetchRegistry: async () => registry,
    resolveModulePath: async () => null,
    detectGo: async () => ({ installed: true }),
    goInstall: async (modulePath) => {
      goCalls.push(modulePath);
      return ok();
    },
    goInstallDir: async () => goBinDir("/Users/example/go/bin"),
    commandOnPath: async () => "/Users/example/go/bin/espn-pp-cli",
    installSkill: async (skillName) => {
      skillCalls.push(skillName);
      return ok();
    },
    stdout: (message) => stdout.push(message),
    stderr: () => {},
  });

  assert.equal(await command(["espn", "--cli-only"]), 0);
  assert.equal(goCalls.length, 1);
  assert.deepEqual(skillCalls, []);
  assert.match(stdout.join("\n"), /binary:/);
  assert.doesNotMatch(stdout.join("\n"), /skill:/);
});

test("install command with --skill-only skips go install and PATH check", async () => {
  const goCalls: string[] = [];
  const skillCalls: string[] = [];
  const detectGoCalls: number[] = [];
  const stdout: string[] = [];
  const command = createInstallCommand({
    fetchRegistry: async () => registry,
    resolveModulePath: async () => null,
    detectGo: async () => {
      detectGoCalls.push(1);
      return { installed: false };
    },
    goInstall: async (modulePath) => {
      goCalls.push(modulePath);
      return ok();
    },
    commandOnPath: async () => null,
    installSkill: async (skillName) => {
      skillCalls.push(skillName);
      return ok();
    },
    stdout: (message) => stdout.push(message),
    stderr: () => {},
  });

  assert.equal(await command(["espn", "--skill-only"]), 0);
  assert.deepEqual(goCalls, []);
  assert.deepEqual(detectGoCalls, []);
  assert.deepEqual(skillCalls, ["pp-espn"]);
  assert.match(stdout.join("\n"), /skill: pp-espn/);
  assert.doesNotMatch(stdout.join("\n"), /binary:/);
});

test("install command with --bin-dir sets GOBIN and reports the chosen binary path", async () => {
  const goCalls: Array<{ modulePath: string; env?: NodeJS.ProcessEnv }> = [];
  const mkdirCalls: string[] = [];
  const stdout: string[] = [];
  const command = createInstallCommand({
    fetchRegistry: async () => registry,
    resolveModulePath: async () => null,
    detectGo: async () => ({ installed: true }),
    goInstall: async (modulePath, _ref, env) => {
      goCalls.push({ modulePath, env });
      return ok();
    },
    goInstallDir: async () => {
      throw new Error("goInstallDir should not be called when --bin-dir is explicit");
    },
    mkdir: async (path) => {
      mkdirCalls.push(path);
    },
    commandOnPath: async () => "/Users/example/.local/bin/espn-pp-cli",
    installSkill: async () => ok(),
    stdout: (message) => stdout.push(message),
    stderr: () => {},
    home: "/Users/example",
    env: { KEEP_ME: "yes", GOBIN: "/old/go/bin" },
  });

  assert.equal(await command(["espn", "--bin-dir", "~/.local/bin"]), 0);
  assert.deepEqual(mkdirCalls, ["/Users/example/.local/bin"]);
  assert.equal(goCalls.length, 1);
  assert.equal(goCalls[0]!.env?.KEEP_ME, "yes");
  assert.equal(goCalls[0]!.env?.GOBIN, "/Users/example/.local/bin");
  assert.match(stdout.join("\n"), /binary: \/Users\/example\/\.local\/bin\/espn-pp-cli/);
});

test("install command defaults Windows installs to LOCALAPPDATA PrintingPress bin", async () => {
  const goCalls: Array<{ env?: NodeJS.ProcessEnv }> = [];
  const mkdirCalls: string[] = [];
  const command = createInstallCommand({
    fetchRegistry: async () => registry,
    resolveModulePath: async () => null,
    detectGo: async () => ({ installed: true }),
    goInstall: async (_modulePath, _ref, env) => {
      goCalls.push({ env });
      return ok();
    },
    goInstallDir: async () => {
      throw new Error("goInstallDir should not be called when default user bin dir is known");
    },
    mkdir: async (path) => {
      mkdirCalls.push(path);
    },
    commandOnPath: async () => "C:\\Users\\you\\AppData\\Local\\Programs\\PrintingPress\\bin\\espn-pp-cli.exe",
    installSkill: async () => ok(),
    stdout: () => {},
    stderr: () => {},
    platform: "win32",
    home: "C:\\Users\\you",
    env: { LOCALAPPDATA: "C:\\Users\\you\\AppData\\Local" },
  });

  assert.equal(await command(["espn"]), 0);
  assert.deepEqual(mkdirCalls, ["C:\\Users\\you\\AppData\\Local\\Programs\\PrintingPress\\bin"]);
  assert.equal(goCalls[0]!.env?.GOBIN, "C:\\Users\\you\\AppData\\Local\\Programs\\PrintingPress\\bin");
});

test("install command rejects --bin-dir with --skill-only", async () => {
  const stderr: string[] = [];
  const command = createInstallCommand({
    fetchRegistry: async () => registry,
    stderr: (message) => stderr.push(message),
  });

  assert.equal(await command(["espn", "--skill-only", "--bin-dir", "/tmp/bin"]), 1);
  assert.match(stderr.join("\n"), /--bin-dir cannot be used with --skill-only/);
});

test("install command rejects --cli-only and --skill-only together", async () => {
  const stderr: string[] = [];
  const command = createInstallCommand({
    fetchRegistry: async () => registry,
    stderr: (message) => stderr.push(message),
  });

  assert.equal(await command(["espn", "--cli-only", "--skill-only"]), 1);
  assert.match(stderr.join("\n"), /mutually exclusive/);
});

test("install command --cli-only with bundle skips every skill", async () => {
  const bundleRegistry: Registry = {
    schema_version: 1,
    entries: [
      { name: "espn", category: "media", api: "ESPN", description: "x", path: "library/media/espn" },
      { name: "flight-goat", category: "travel", api: "FlightGoat", description: "x", path: "library/travel/flightgoat" },
      { name: "movie-goat", category: "media", api: "MovieGoat", description: "x", path: "library/media/movie-goat" },
      { name: "recipe-goat", category: "food", api: "RecipeGoat", description: "x", path: "library/food/recipe-goat" },
    ],
  };
  const goCalls: string[] = [];
  const skillCalls: string[] = [];
  const command = createInstallCommand({
    fetchRegistry: async () => bundleRegistry,
    resolveModulePath: async () => null,
    detectGo: async () => ({ installed: true }),
    goInstall: async (modulePath) => {
      goCalls.push(modulePath);
      return ok();
    },
    goInstallDir: async () => goBinDir("/Users/example/go/bin"),
    commandOnPath: async (binary) => `/Users/example/go/bin/${binary}`,
    installSkill: async (skillName) => {
      skillCalls.push(skillName);
      return ok();
    },
    stdout: () => {},
    stderr: () => {},
  });

  assert.equal(await command(["starter-pack", "--cli-only"]), 0);
  assert.equal(goCalls.length, 4);
  assert.deepEqual(skillCalls, []);
});

test("install command uses go.mod module path when it differs from registry path", async () => {
  const hubspotRegistry: Registry = {
    schema_version: 2,
    entries: [
      {
        name: "hubspot-pp-cli",
        category: "sales-and-crm",
        api: "HubSpot",
        description: "CRM",
        path: "library/sales-and-crm/hubspot",
      },
    ],
  };
  const goCalls: string[] = [];
  const command = createInstallCommand({
    fetchRegistry: async () => hubspotRegistry,
    resolveModulePath: async () =>
      "github.com/mvanhorn/printing-press-library/library/sales-and-crm/hubspot-pp-cli",
    detectGo: async () => ({ installed: true }),
    goInstall: async (modulePath) => {
      goCalls.push(modulePath);
      return ok();
    },
    goInstallDir: async () => goBinDir("/Users/example/go/bin"),
    commandOnPath: async () => "/Users/example/go/bin/hubspot-pp-cli",
    installSkill: async () => ok(),
    stdout: () => {},
    stderr: () => {},
  });

  assert.equal(await command(["hubspot"]), 0);
  assert.deepEqual(goCalls, [
    "github.com/mvanhorn/printing-press-library/library/sales-and-crm/hubspot-pp-cli/cmd/hubspot-pp-cli",
  ]);
});

test("install command warns loudly when a stale binary earlier in PATH shadows the freshly built one", async () => {
  const stdout: string[] = [];
  const stderr: string[] = [];
  const command = createInstallCommand({
    fetchRegistry: async () => registry,
    resolveModulePath: async () => null,
    detectGo: async () => ({ installed: true }),
    goInstall: async () => ok(),
    goInstallDir: async () => goBinDir("/Users/ada/go/bin"),
    commandOnPath: async () => "/opt/homebrew/bin/espn-pp-cli",
    home: "/Users/ada",
    env: {},
    mkdir: async () => {},
    realpath: async (path) => path,
    installSkill: async () => ok(),
    stdout: (message) => stdout.push(message),
    stderr: (message) => stderr.push(message),
  });

  assert.equal(await command(["espn"]), 0);
  assert.match(stderr.join("\n"), /WARNING/);
  assert.match(stderr.join("\n"), /shadow/);
  assert.match(stderr.join("\n"), /\/Users\/ada\/\.local\/bin\/espn-pp-cli/);
  assert.match(stderr.join("\n"), /\/opt\/homebrew\/bin\/espn-pp-cli/);
  // Success line reports the freshly installed path, not the shadow.
  assert.match(stdout.join("\n"), /binary: \/Users\/ada\/\.local\/bin\/espn-pp-cli/);
  assert.match(stdout.join("\n"), /shadowed by: \/opt\/homebrew\/bin\/espn-pp-cli/);
});

test("install command does not warn when PATH already resolves to the freshly installed binary", async () => {
  const stdout: string[] = [];
  const stderr: string[] = [];
  const command = createInstallCommand({
    fetchRegistry: async () => registry,
    resolveModulePath: async () => null,
    detectGo: async () => ({ installed: true }),
    goInstall: async () => ok(),
    goInstallDir: async () => goBinDir("/Users/example/go/bin"),
    commandOnPath: async () => "/Users/example/.local/bin/espn-pp-cli",
    home: "/Users/example",
    env: {},
    mkdir: async () => {},
    installSkill: async () => ok(),
    stdout: (message) => stdout.push(message),
    stderr: (message) => stderr.push(message),
  });

  assert.equal(await command(["espn"]), 0);
  assert.doesNotMatch(stderr.join("\n"), /shadow/);
  assert.doesNotMatch(stdout.join("\n"), /shadowed by/);
});

test("install command includes shadow info in JSON output", async () => {
  const stdout: string[] = [];
  const command = createInstallCommand({
    fetchRegistry: async () => registry,
    resolveModulePath: async () => null,
    detectGo: async () => ({ installed: true }),
    goInstall: async () => ok(),
    goInstallDir: async () => goBinDir("/Users/ada/go/bin"),
    commandOnPath: async () => "/opt/homebrew/bin/espn-pp-cli",
    home: "/Users/ada",
    env: {},
    mkdir: async () => {},
    realpath: async (path) => path,
    installSkill: async () => ok(),
    stdout: (message) => stdout.push(message),
    stderr: () => {},
  });

  assert.equal(await command(["espn", "--json"]), 0);
  const json = JSON.parse(stdout[0]!);
  assert.equal(json.ok, true);
  assert.equal(json.binaryPath, "/Users/ada/.local/bin/espn-pp-cli");
  assert.equal(json.installedPath, "/Users/ada/.local/bin/espn-pp-cli");
  assert.equal(json.shadowedBy, "/opt/homebrew/bin/espn-pp-cli");
});

test("install command includes PATH warning in JSON output", async () => {
  const stdout: string[] = [];
  const command = createInstallCommand({
    fetchRegistry: async () => registry,
    resolveModulePath: async () => null,
    detectGo: async () => ({ installed: true }),
    goInstall: async () => ok(),
    goInstallDir: async () => goBinDir("/Users/example/go/bin"),
    commandOnPath: async () => null,
    home: "/Users/example",
    env: {},
    mkdir: async () => {},
    installSkill: async () => ok(),
    stdout: (message) => stdout.push(message),
    stderr: () => {},
  });

  assert.equal(await command(["espn", "--json"]), 0);
  const json = JSON.parse(stdout[0]!);
  assert.equal(json.ok, true);
  assert.equal(json.binaryPath, "/Users/example/.local/bin/espn-pp-cli");
  assert.equal(json.installedPath, "/Users/example/.local/bin/espn-pp-cli");
  assert.equal(json.pathWarning, "not_on_path");
  assert.equal(json.skill, "pp-espn");
});

test("install command does not warn when PATH hit is a symlink to the freshly installed binary", async () => {
  const stdout: string[] = [];
  const stderr: string[] = [];
  const realpathCalls: string[] = [];
  const command = createInstallCommand({
    fetchRegistry: async () => registry,
    resolveModulePath: async () => null,
    detectGo: async () => ({ installed: true }),
    goInstall: async () => ok(),
    goInstallDir: async () => goBinDir("/Users/example/go/bin"),
    commandOnPath: async () => "/usr/local/bin/espn-pp-cli",
    home: "/Users/example",
    env: {},
    mkdir: async () => {},
    realpath: async (path) => {
      realpathCalls.push(path);
      if (path === "/usr/local/bin/espn-pp-cli") {
        return "/Users/example/.local/bin/espn-pp-cli";
      }
      return path;
    },
    installSkill: async () => ok(),
    stdout: (message) => stdout.push(message),
    stderr: (message) => stderr.push(message),
  });

  assert.equal(await command(["espn"]), 0);
  assert.deepEqual(realpathCalls.sort(), [
    "/Users/example/.local/bin/espn-pp-cli",
    "/usr/local/bin/espn-pp-cli",
  ]);
  assert.doesNotMatch(stderr.join("\n"), /shadow/);
  assert.doesNotMatch(stdout.join("\n"), /shadowed by/);
  assert.match(stdout.join("\n"), /binary: \/Users\/example\/\.local\/bin\/espn-pp-cli/);
});

test("install command falls back to PATH match when go env returns no install dir", async () => {
  const stdout: string[] = [];
  const stderr: string[] = [];
  const command = createInstallCommand({
    fetchRegistry: async () => registry,
    resolveModulePath: async () => null,
    detectGo: async () => ({ installed: true }),
    goInstall: async () => ok(),
    goInstallDir: async () => ({ binDir: null, gobin: "", gopath: "" }),
    commandOnPath: async () => "/usr/local/bin/espn-pp-cli",
    home: undefined,
    env: {},
    mkdir: async () => {},
    installSkill: async () => ok(),
    stdout: (message) => stdout.push(message),
    stderr: (message) => stderr.push(message),
  });

  assert.equal(await command(["espn"]), 0);
  // No GOBIN/GOPATH means we can't compare paths — skip the shadow warning.
  assert.doesNotMatch(stderr.join("\n"), /shadow/);
  assert.match(stdout.join("\n"), /binary: \/usr\/local\/bin\/espn-pp-cli/);
});

test("install command calls goInstallDir once per invocation regardless of bundle size", async () => {
  const bundleRegistry: Registry = {
    schema_version: 1,
    entries: [
      { name: "espn", category: "media", api: "ESPN", description: "x", path: "library/media/espn" },
      { name: "flight-goat", category: "travel", api: "FlightGoat", description: "x", path: "library/travel/flightgoat" },
      { name: "movie-goat", category: "media", api: "MovieGoat", description: "x", path: "library/media/movie-goat" },
      { name: "recipe-goat", category: "food", api: "RecipeGoat", description: "x", path: "library/food/recipe-goat" },
    ],
  };
  let goInstallDirCalls = 0;
  const command = createInstallCommand({
    fetchRegistry: async () => bundleRegistry,
    resolveModulePath: async () => null,
    detectGo: async () => ({ installed: true }),
    goInstall: async () => ok(),
    goInstallDir: async () => {
      goInstallDirCalls++;
      return goBinDir("/Users/example/go/bin");
    },
    commandOnPath: async (binary) => `/Users/example/.local/bin/${binary}`,
    home: "/Users/example",
    env: {},
    mkdir: async () => {},
    realpath: async (path) => path,
    installSkill: async () => ok(),
    stdout: () => {},
    stderr: () => {},
  });

  assert.equal(await command(["starter-pack"]), 0);
  // Default user-bin installs do not need go env path discovery.
  assert.equal(goInstallDirCalls, 0);
});

test("install command points at the install dir when nothing is on PATH", async () => {
  const stderr: string[] = [];
  const command = createInstallCommand({
    fetchRegistry: async () => registry,
    resolveModulePath: async () => null,
    detectGo: async () => ({ installed: true }),
    goInstall: async () => ok(),
    goInstallDir: async () => goBinDir("/Users/example/go/bin"),
    commandOnPath: async () => null,
    home: "/Users/example",
    env: {},
    mkdir: async () => {},
    installSkill: async () => ok(),
    stdout: () => {},
    stderr: (message) => stderr.push(message),
  });

  assert.equal(await command(["espn"]), 0);
  // The error message names the specific path the user needs to add to PATH.
  assert.match(stderr.join("\n"), /\/Users\/example\/\.local\/bin\/espn-pp-cli/);
});

function threeEntryRegistry(): Registry {
  return {
    schema_version: 2,
    entries: [
      { name: "espn", category: "sports", api: "ESPN", description: "Sports scores", path: "library/sports/espn" },
      { name: "linear", category: "project-management", api: "Linear", description: "Issues", path: "library/project-management/linear" },
      { name: "opensnow", category: "sports", api: "OpenSnow", description: "Snow forecasts", path: "library/sports/opensnow" },
    ],
  };
}

function bulkDeps(overrides: {
  goInstall?: (modulePath: string) => Promise<RunResult>;
  installSkill?: (skillName: string) => Promise<RunResult>;
  stdout?: (message: string) => void;
  stderr?: (message: string) => void;
  detectGoCalls?: { count: number };
}) {
  return createInstallCommand({
    fetchRegistry: async () => threeEntryRegistry(),
    resolveModulePath: async () => null,
    detectGo: async () => {
      if (overrides.detectGoCalls) {
        overrides.detectGoCalls.count++;
      }
      return { installed: true };
    },
    goInstall: async (modulePath) => (overrides.goInstall ? overrides.goInstall(modulePath) : ok()),
    goInstallDir: async () => goBinDir("/Users/example/go/bin"),
    commandOnPath: async (binary) => `/Users/example/go/bin/${binary}`,
    mkdir: async () => {},
    installSkill: async (skillName) => (overrides.installSkill ? overrides.installSkill(skillName) : ok()),
    stdout: overrides.stdout ?? (() => {}),
    stderr: overrides.stderr ?? (() => {}),
    home: "/Users/example",
    env: {},
  });
}

test("install command runs multi-name installs concurrently", async () => {
  let inFlight = 0;
  let maxInFlight = 0;
  const command = bulkDeps({
    goInstall: async () => {
      inFlight++;
      maxInFlight = Math.max(maxInFlight, inFlight);
      // Yield so overlapping workers can enter before this one leaves.
      await new Promise((resolve) => setTimeout(resolve, 10));
      inFlight--;
      return ok();
    },
  });

  assert.equal(await command(["espn", "linear", "opensnow", "--cli-only"]), 0);
  assert.ok(maxInFlight > 1, `expected concurrent go installs, saw max in-flight ${maxInFlight}`);
});

test("install command serializes skill installs during bulk installs", async () => {
  let goInFlight = 0;
  let maxGoInFlight = 0;
  let skillInFlight = 0;
  let maxSkillInFlight = 0;
  const installedSkills: string[] = [];
  const command = bulkDeps({
    goInstall: async () => {
      goInFlight++;
      maxGoInFlight = Math.max(maxGoInFlight, goInFlight);
      await new Promise((resolve) => setTimeout(resolve, 10));
      goInFlight--;
      return ok();
    },
    installSkill: async (skillName) => {
      skillInFlight++;
      maxSkillInFlight = Math.max(maxSkillInFlight, skillInFlight);
      await new Promise((resolve) => setTimeout(resolve, 10));
      installedSkills.push(skillName);
      skillInFlight--;
      return ok();
    },
  });

  assert.equal(await command(["espn", "linear", "opensnow"]), 0);
  assert.ok(maxGoInFlight > 1, `expected concurrent go installs, saw max in-flight ${maxGoInFlight}`);
  assert.equal(maxSkillInFlight, 1);
  assert.deepEqual(new Set(installedSkills), new Set(["pp-espn", "pp-linear", "pp-opensnow"]));
});

test("install command --all installs every catalog entry once", async () => {
  const installed: string[] = [];
  const command = bulkDeps({
    goInstall: async (modulePath) => {
      installed.push(modulePath);
      return ok();
    },
  });

  assert.equal(await command(["--all", "espn", "--cli-only"]), 0);
  assert.equal(installed.length, 3);
  assert.equal(new Set(installed).size, 3);
});

test("install command --category installs that category only", async () => {
  const installed: string[] = [];
  const stdout: string[] = [];
  const command = bulkDeps({
    goInstall: async (modulePath) => {
      installed.push(modulePath);
      return ok();
    },
    stdout: (message) => stdout.push(message),
  });

  assert.equal(await command(["--category", "sports", "--cli-only"]), 0);
  assert.equal(installed.length, 2);
  assert.match(stdout.join("\n"), /Category "sports"/);
});

test("install command --category rejects unknown categories with the known list", async () => {
  const stderr: string[] = [];
  const command = bulkDeps({ stderr: (message) => stderr.push(message) });

  assert.equal(await command(["--category", "nope"]), 1);
  assert.match(stderr.join("\n"), /No CLIs in category "nope"/);
  assert.match(stderr.join("\n"), /project-management, sports/);
});

test("install command emits progress lines for bulk installs", async () => {
  const stderr: string[] = [];
  const command = bulkDeps({ stderr: (message) => stderr.push(message) });

  assert.equal(await command(["espn", "linear", "--cli-only"]), 0);
  const progress = stderr.filter((line) => /^\[\d\/2\] /.test(line));
  assert.equal(progress.length, 2);
  assert.match(progress.join("\n"), /\[2\/2\] \w+: ok/);
});

test("install command checks Go once for a bulk install and fails fast when missing", async () => {
  const stderr: string[] = [];
  const goInstallCalls: string[] = [];
  const command = createInstallCommand({
    fetchRegistry: async () => threeEntryRegistry(),
    resolveModulePath: async () => null,
    detectGo: async () => ({ installed: false }),
    goInstall: async (modulePath) => {
      goInstallCalls.push(modulePath);
      return ok();
    },
    goInstallDir: async () => goBinDir("/Users/example/go/bin"),
    commandOnPath: async () => null,
    mkdir: async () => {},
    installSkill: async () => ok(),
    stdout: () => {},
    stderr: (message) => stderr.push(message),
    home: "/Users/example",
    env: {},
    platform: "darwin",
  });

  assert.equal(await command(["espn", "linear", "opensnow"]), 1);
  assert.equal(goInstallCalls.length, 0);
  assert.equal(stderr.filter((line) => /Go is required/.test(line)).length, 1);
});

test("install command keeps per-CLI output contiguous under concurrency", async () => {
  const stdout: string[] = [];
  const command = bulkDeps({
    goInstall: async (modulePath) => {
      // Reverse-order delays force out-of-order completion.
      const delay = modulePath.includes("espn") ? 30 : modulePath.includes("linear") ? 15 : 1;
      await new Promise((resolve) => setTimeout(resolve, delay));
      return ok();
    },
    stdout: (message) => stdout.push(message),
  });

  assert.equal(await command(["espn", "linear", "opensnow"]), 0);
  const order = stdout.filter((line) => /^Installed [a-z]/.test(line)).map((line) => line.split(" ")[1]);
  assert.deepEqual(order, ["espn", "linear", "opensnow"]);
});
