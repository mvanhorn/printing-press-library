import test from "node:test";
import assert from "node:assert/strict";
import { createInstallCommand } from "../src/commands/install.js";
import type { GoDetection } from "../src/go.js";
import type { RunResult } from "../src/process.js";
import type { Registry } from "../src/registry.js";

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
    commandOnPath: async () => "/Users/example/go/bin/espn-pp-cli",
    installSkill: async (skillName, agents) => {
      skillCalls.push({ skillName, agents });
      return ok();
    },
    stdout: (message) => stdout.push(message),
    stderr: () => {},
  });

  assert.equal(await command(["espn", "--agent", "claude-code"]), 0);

  assert.deepEqual(goCalls, [
    {
      modulePath:
        "github.com/mvanhorn/printing-press-library/library/sports/espn/cmd/espn-pp-cli",
      ref: "latest",
      env: undefined,
    },
  ]);
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
    commandOnPath: async () => "/Users/example/go/bin/espn-pp-cli",
    installSkill: async () => ok(),
    stdout: () => {},
    stderr: (message) => stderr.push(message),
  });

  assert.equal(await command(["espn"]), 1);
  assert.deepEqual(refs, ["latest"]);
  assert.match(stderr.join("\n"), /go install failed/);
});

test("install command stops when binary is not on PATH", async () => {
  const skillCalls: string[] = [];
  const stderr: string[] = [];
  const command = createInstallCommand({
    fetchRegistry: async () => registry,
    resolveModulePath: async () => null,
    detectGo: async () => ({ installed: true }),
    goInstall: async () => ok(),
    commandOnPath: async () => null,
    installSkill: async () => {
      skillCalls.push("skill");
      return ok();
    },
    stderr: (message) => stderr.push(message),
  });

  assert.equal(await command(["espn"]), 1);
  assert.deepEqual(skillCalls, []);
  assert.match(stderr.join("\n"), /not on PATH/);
});

test("install command reports skill install failure without hiding binary", async () => {
  const stderr: string[] = [];
  const command = createInstallCommand({
    fetchRegistry: async () => registry,
    resolveModulePath: async () => null,
    detectGo: async () => ({ installed: true }),
    goInstall: async () => ok(),
    commandOnPath: async () => "/Users/example/go/bin/espn-pp-cli",
    installSkill: async () => fail("network down"),
    stderr: (message) => stderr.push(message),
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
    commandOnPath: async () => "/Users/example/go/bin/espn-pp-cli",
    installSkill: async () => ok(),
    stdout: (message) => stdout.push(message),
    stderr: () => {},
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
    commandOnPath: async (binary) => `/Users/example/go/bin/${binary}`,
    installSkill: async (skillName) => {
      skills.push(skillName);
      return ok();
    },
    stdout: (message) => stdout.push(message),
    stderr: () => {},
  });

  assert.equal(await command(["espn", "linear"]), 0);
  assert.equal(installed.length, 2);
  assert.deepEqual(skills, ["pp-espn", "pp-linear"]);
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
    commandOnPath: async (binary) => `/Users/example/go/bin/${binary}`,
    installSkill: async () => ok(),
    stdout: (message) => stdout.push(message),
    stderr: () => {},
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
    commandOnPath: async (binary) => `/Users/example/go/bin/${binary}`,
    installSkill: async () => ok(),
    stdout: (message) => stdout.push(message),
    stderr: () => {},
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
