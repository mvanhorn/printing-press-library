import test from "node:test";
import assert from "node:assert/strict";
import { createListCommand } from "../src/commands/list.js";
import { createSearchCommand, searchRegistry } from "../src/commands/search.js";
import { createUninstallCommand } from "../src/commands/uninstall.js";
import { createUpdateCommand } from "../src/commands/update.js";
import { CLI_COMMAND_NAME, commandPrefixForInvocation, NPX_COMMAND_PREFIX } from "../src/constants.js";
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
    {
      name: "hotel-tonight",
      category: "travel",
      api: "HotelTonight",
      description: "Last-minute hotel deals",
      path: "library/travel/hotel-tonight",
    },
    {
      name: "cal-com",
      category: "productivity",
      api: "Cal.com",
      description: "Scheduling and booking links",
      path: "library/productivity/cal-com",
    },
    {
      name: "booking-com",
      category: "travel",
      api: "Booking.com",
      description: "Every Booking.com workflow",
      search_terms: ["Search Booking.com hotels, scrape details and reviews, watch prices over time."],
      path: "library/travel/booking-com",
    },
  ],
};

const ok = (stdout = ""): RunResult => ({ code: 0, stdout, stderr: "" });

test("list command reports catalog CLIs by default", async () => {
  const stdout: string[] = [];
  const command = createListCommand({
    commandPrefix: CLI_COMMAND_NAME,
    fetchRegistry: async () => registry,
    stdout: (message) => stdout.push(message),
  });

  assert.equal(await command([]), 0);
  assert.match(stdout.join("\n"), /espn-pp-cli/);
  assert.match(stdout.join("\n"), /dominos-pp-cli/);
  assert.match(stdout.join("\n"), /install: printing-press-library install espn/);
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

test("list command suggests the current wrapper command when no installed CLIs are detected", async () => {
  const stdout: string[] = [];
  const command = createListCommand({
    commandPrefix: CLI_COMMAND_NAME,
    fetchRegistry: async () => registry,
    commandOnPath: async () => null,
    stdout: (message) => stdout.push(message),
  });

  assert.equal(await command(["--installed"]), 0);
  assert.match(stdout.join("\n"), /printing-press-library search <query>/);
  assert.match(stdout.join("\n"), /printing-press-library install <name>/);
});

test("search command ranks registry matches", async () => {
  const stdout: string[] = [];
  const command = createSearchCommand({
    commandPrefix: CLI_COMMAND_NAME,
    fetchRegistry: async () => registry,
    stdout: (message) => stdout.push(message),
  });

  assert.equal(await command(["pizza"]), 0);
  assert.match(stdout.join("\n"), /dominos-pp-cli/);
  assert.match(stdout.join("\n"), /install: printing-press-library install dominos-pp-cli/);
});

test("catalog hints preserve npx when the wrapper is running through npx", async () => {
  const stdout: string[] = [];
  const command = createSearchCommand({
    commandPrefix: NPX_COMMAND_PREFIX,
    fetchRegistry: async () => registry,
    stdout: (message) => stdout.push(message),
  });

  assert.equal(await command(["pizza"]), 0);
  assert.match(stdout.join("\n"), /install: npx -y @mvanhorn\/printing-press-library install dominos-pp-cli/);
});

test("search usage follows the current wrapper command", async () => {
  const stderr: string[] = [];
  const command = createSearchCommand({
    commandPrefix: NPX_COMMAND_PREFIX,
    stderr: (message) => stderr.push(message),
  });

  assert.equal(await command([]), 1);
  assert.match(stderr.join("\n"), /Usage: npx -y @mvanhorn\/printing-press-library search <query> \[--json\]/);
});

test("command prefix follows the invocation source", () => {
  assert.equal(commandPrefixForInvocation("/opt/homebrew/bin/printing-press-library", {}), "printing-press-library");
  assert.equal(
    commandPrefixForInvocation("/Users/me/.npm/_npx/123/node_modules/.bin/printing-press-library", {}),
    NPX_COMMAND_PREFIX,
  );
  assert.equal(commandPrefixForInvocation("/tmp/printing-press-library", { npm_command: "exec" }), NPX_COMMAND_PREFIX);
});

test("search command normalizes punctuation and plural queries", async () => {
  const stdout: string[] = [];
  const command = createSearchCommand({
    fetchRegistry: async () => registry,
    stdout: (message) => stdout.push(message),
  });

  assert.equal(await command(["hotels"]), 0);
  assert.match(stdout.join("\n"), /hotel-tonight-pp-cli/);
  assert.match(stdout.join("\n"), /booking-com-pp-cli/);

  stdout.length = 0;
  assert.equal(await command(["cal.com"]), 0);
  assert.match(stdout.join("\n"), /cal-com-pp-cli/);
});

test("search command ignores shared pp-cli binary suffix tokens", async () => {
  assert.deepEqual(searchRegistry(registry.entries, "a"), []);
  assert.deepEqual(searchRegistry(registry.entries, "a-b"), []);
  assert.deepEqual(searchRegistry(registry.entries, "t"), []);
  assert.deepEqual(searchRegistry(registry.entries, "cli"), []);
  assert.deepEqual(searchRegistry(registry.entries, "pp"), []);
  assert.deepEqual(searchRegistry(registry.entries, "pp-cli"), []);
  assert.equal(searchRegistry(registry.entries, "cal")[0]?.name, "cal-com");
  assert.equal(searchRegistry(registry.entries, "dominos-pp-cli")[0]?.name, "dominos-pp-cli");
  assert.equal(searchRegistry(registry.entries, "hotels-pp-cli")[0]?.name, "hotel-tonight");
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
