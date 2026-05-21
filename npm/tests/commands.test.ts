import test from "node:test";
import assert from "node:assert/strict";
import { createListCommand } from "../src/commands/list.js";
import { createSearchCommand } from "../src/commands/search.js";
import { createUninstallCommand } from "../src/commands/uninstall.js";
import { createUpdateCommand } from "../src/commands/update.js";
import type { RunResult } from "../src/process.js";
import type { Registry } from "../src/registry.js";

const registry: Registry = {
  schema_version: 1,
  entries: [
    {
      name: "espn",
      category: "sports",
      api: "ESPN",
      description: "Live sports scores",
      path: "library/sports/espn",
    },
    {
      name: "dominos-pp-cli",
      category: "commerce",
      api: "Dominos",
      description: "Pizza ordering",
      path: "library/commerce/dominos",
    },
  ],
};

const ok = (stdout = ""): RunResult => ({ code: 0, stdout, stderr: "" });

test("list command reports catalog CLIs by default", async () => {
  const stdout: string[] = [];
  const command = createListCommand({
    fetchRegistry: async () => registry,
    stdout: (message) => stdout.push(message),
  });

  assert.equal(await command([]), 0);
  assert.match(stdout.join("\n"), /espn-pp-cli/);
  assert.match(stdout.join("\n"), /dominos-pp-cli/);
});

test("list command can filter catalog CLIs by category", async () => {
  const stdout: string[] = [];
  const command = createListCommand({
    fetchRegistry: async () => registry,
    stdout: (message) => stdout.push(message),
  });

  assert.equal(await command(["--category", "sports"]), 0);
  assert.match(stdout.join("\n"), /espn-pp-cli/);
  assert.doesNotMatch(stdout.join("\n"), /dominos/);
});

test("list command reports installed CLIs with --installed", async () => {
  const stdout: string[] = [];
  const command = createListCommand({
    fetchRegistry: async () => registry,
    commandOnPath: async (binary) => (binary === "espn-pp-cli" ? "/bin/espn-pp-cli" : null),
    runner: async () => ok("espn-pp-cli version 1.0.0\n"),
    stdout: (message) => stdout.push(message),
  });

  assert.equal(await command(["--installed"]), 0);
  assert.match(stdout.join("\n"), /espn-pp-cli/);
  assert.doesNotMatch(stdout.join("\n"), /dominos/);
});

test("list command can filter installed CLIs by category", async () => {
  const stdout: string[] = [];
  const checkedBinaries: string[] = [];
  const command = createListCommand({
    fetchRegistry: async () => registry,
    commandOnPath: async (binary) => {
      checkedBinaries.push(binary);
      return binary === "espn-pp-cli" ? "/bin/espn-pp-cli" : null;
    },
    runner: async () => ok("espn-pp-cli version 1.0.0\n"),
    stdout: (message) => stdout.push(message),
  });

  assert.equal(await command(["--installed", "--category", "sports"]), 0);
  assert.deepEqual(checkedBinaries, ["espn-pp-cli"]);
  assert.match(stdout.join("\n"), /espn-pp-cli/);
  assert.doesNotMatch(stdout.join("\n"), /dominos/);
});

test("search command ranks registry matches", async () => {
  const stdout: string[] = [];
  const command = createSearchCommand({
    fetchRegistry: async () => registry,
    stdout: (message) => stdout.push(message),
  });

  assert.equal(await command(["pizza"]), 0);
  assert.match(stdout.join("\n"), /dominos-pp-cli/);
});

test("update command refreshes detected installed CLIs", async () => {
  const installs: string[][] = [];
  const command = createUpdateCommand({
    fetchRegistry: async () => registry,
    commandOnPath: async (binary) => (binary === "espn-pp-cli" ? "/bin/espn-pp-cli" : null),
    install: async (args) => {
      installs.push(args);
      return 0;
    },
  });

  assert.equal(await command(["--agent", "claude-code"]), 0);
  assert.deepEqual(installs, [["espn", "--agent", "claude-code"]]);
});

test("uninstall command requires --yes", async () => {
  const stderr: string[] = [];
  const command = createUninstallCommand({
    fetchRegistry: async () => registry,
    stderr: (message) => stderr.push(message),
  });

  assert.equal(await command(["espn"]), 1);
  assert.match(stderr.join("\n"), /without --yes/);
});

test("uninstall command removes binary and skill", async () => {
  const removedFiles: string[] = [];
  const removedSkills: Array<{ skillName: string; agents: string[] }> = [];
  const stdout: string[] = [];
  const command = createUninstallCommand({
    fetchRegistry: async () => registry,
    commandOnPath: async () => "/bin/espn-pp-cli",
    removeFile: async (path) => {
      removedFiles.push(path);
    },
    removeSkill: async (skillName, agents) => {
      removedSkills.push({ skillName, agents });
      return ok();
    },
    stdout: (message) => stdout.push(message),
  });

  assert.equal(await command(["espn", "--yes", "--agent", "claude-code"]), 0);
  assert.deepEqual(removedFiles, ["/bin/espn-pp-cli"]);
  assert.deepEqual(removedSkills, [{ skillName: "pp-espn", agents: ["claude-code"] }]);
  assert.match(stdout.join("\n"), /Uninstalled espn/);
});
