import test from "node:test";
import assert from "node:assert/strict";
import { createListCommand } from "../src/commands/list.js";
import { createSearchCommand, searchRegistry } from "../src/commands/search.js";
import { createUninstallCommand } from "../src/commands/uninstall.js";
import { createUpdateCommand } from "../src/commands/update.js";
import { run } from "../src/cli.js";
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
      release: {
        cli_name: "espn-pp-cli",
        version: "2026.6.1",
        released_at: "2026-06-01T00:00:00Z",
        source_commit: "0123456789abcdef0123456789abcdef01234567",
      },
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

test("list --json includes catalog release metadata", async () => {
  const stdout: string[] = [];
  const command = createListCommand({
    fetchRegistry: async () => registry,
    stdout: (message) => stdout.push(message),
  });

  assert.equal(await command(["--category", "sports", "--json"]), 0);
  const entries = JSON.parse(stdout.join("\n"));
  assert.equal(entries[0].release.version, "2026.6.1");
  assert.equal(entries[0].release.cli_name, "espn-pp-cli");
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

test("search --json includes catalog release metadata", async () => {
  const stdout: string[] = [];
  const command = createSearchCommand({
    fetchRegistry: async () => registry,
    stdout: (message) => stdout.push(message),
  });

  assert.equal(await command(["espn", "--json"]), 0);
  const entries = JSON.parse(stdout.join("\n"));
  assert.equal(entries[0].release.version, "2026.6.1");
  assert.equal(entries[0].release.source_commit, "0123456789abcdef0123456789abcdef01234567");
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
    createInstall: () => async (args) => {
      installs.push(args);
      return 0;
    },
  });

  assert.equal(await command(["--agent", "claude-code"]), 0);
  assert.deepEqual(installs, [["espn", "--agent", "claude-code"]]);
});

test("update command forwards --bin-dir to detected installed CLIs", async () => {
  const installs: string[][] = [];
  const command = createUpdateCommand({
    fetchRegistry: async () => registry,
    commandOnPath: async (binary) => (binary === "espn-pp-cli" ? "/bin/espn-pp-cli" : null),
    createInstall: () => async (args) => {
      installs.push(args);
      return 0;
    },
  });

  assert.equal(await command(["--agent", "claude-code", "--bin-dir", "/Users/example/.local/bin"]), 0);
  assert.deepEqual(installs, [["espn", "--agent", "claude-code", "--bin-dir", "/Users/example/.local/bin"]]);
});

test("reinstall dispatches to the update handler rather than the unknown-command path", async () => {
  // `reinstall --bogus` fails in the update arg parser before any network call,
  // which proves the alias routes to `update` (and not to "Unknown command").
  const errors: string[] = [];
  const originalError = console.error;
  console.error = (message) => errors.push(String(message));
  let code: number;
  try {
    code = await run(["reinstall", "--bogus-flag"]);
  } finally {
    console.error = originalError;
  }

  assert.equal(code, 1);
  assert.doesNotMatch(errors.join("\n"), /Unknown command/);
  assert.match(errors.join("\n"), /Unknown option: --bogus-flag/);
});

test("help lists the reinstall command", async () => {
  const lines: string[] = [];
  const originalLog = console.log;
  console.log = (message) => lines.push(String(message));
  try {
    assert.equal(await run(["--help"]), 0);
  } finally {
    console.log = originalLog;
  }

  assert.match(lines.join("\n"), /reinstall \[name\]/);
});

test("update refreshes detected CLIs concurrently and flushes output in catalog order", async () => {
  // espn (entry 0) and cal-com (entry 3) are "installed"; dominos/hotel-tonight/booking are not.
  let inFlight = 0;
  let maxInFlight = 0;
  const stdout: string[] = [];
  const command = createUpdateCommand({
    fetchRegistry: async () => registry,
    commandOnPath: async (binary) =>
      binary === "espn-pp-cli" || binary === "cal-com-pp-cli" ? `/bin/${binary}` : null,
    createInstall: (io) => async (args) => {
      inFlight++;
      maxInFlight = Math.max(maxInFlight, inFlight);
      const name = args[0]!;
      // Emit two lines so interleaving (if it regressed) would be observable.
      io.stdout(`Installed ${name}`);
      await Promise.resolve();
      io.stdout(`  binary: /bin/${name}-pp-cli`);
      inFlight--;
      return 0;
    },
    stdout: (message) => stdout.push(message),
  });

  assert.equal(await command([]), 0);
  // Both detected CLIs ran with overlap (concurrent, not serialized one-at-a-time).
  assert.equal(maxInFlight, 2);
  // Output stays grouped per CLI and ordered by catalog position (espn before cal-com).
  assert.deepEqual(stdout, [
    "Installed espn",
    "  binary: /bin/espn-pp-cli",
    "Installed cal-com",
    "  binary: /bin/cal-com-pp-cli",
  ]);
});

test("update preserves stdout/stderr emission order within a CLI block", async () => {
  // install emits a stderr warning *before* its stdout success lines; buffering
  // must replay them in that order, not group all stdout then all stderr.
  const log: string[] = [];
  const command = createUpdateCommand({
    fetchRegistry: async () => registry,
    commandOnPath: async (binary) => (binary === "espn-pp-cli" ? "/bin/espn-pp-cli" : null),
    createInstall: (io) => async () => {
      io.stderr("warning: shadowed by an older binary");
      io.stdout("Installed espn");
      return 0;
    },
    stdout: (message) => log.push(`out:${message}`),
    stderr: (message) => log.push(`err:${message}`),
  });

  assert.equal(await command([]), 0);
  assert.deepEqual(log, ["err:warning: shadowed by an older binary", "out:Installed espn"]);
});

test("update skips a CLI whose PATH probe throws instead of aborting the run", async () => {
  const installed: string[] = [];
  const command = createUpdateCommand({
    fetchRegistry: async () => registry,
    commandOnPath: async (binary) => {
      if (binary === "dominos-pp-cli") throw new Error("which exploded");
      return binary === "espn-pp-cli" || binary === "cal-com-pp-cli" ? `/bin/${binary}` : null;
    },
    createInstall: () => async (args) => {
      installed.push(args[0]!);
      return 0;
    },
  });

  // The throwing probe is treated as "not installed"; the others still update.
  assert.equal(await command([]), 0);
  assert.deepEqual(installed.sort(), ["cal-com", "espn"]);
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
