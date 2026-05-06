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

test("install command retries go install at main when latest fails", async () => {
  const refs: string[] = [];
  const command = createInstallCommand({
    fetchRegistry: async () => registry,
    resolveModulePath: async () => null,
    detectGo: async () => ({ installed: true }),
    goInstall: async (_modulePath, ref) => {
      refs.push(ref);
      return ref === "latest" ? fail("proxy miss") : ok();
    },
    commandOnPath: async () => "/Users/example/go/bin/espn-pp-cli",
    installSkill: async () => ok(),
    stdout: () => {},
    stderr: () => {},
  });

  assert.equal(await command(["espn"]), 0);
  assert.deepEqual(refs, ["latest", "main"]);
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
