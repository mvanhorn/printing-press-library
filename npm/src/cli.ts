import { readFile } from "node:fs/promises";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { installCommand } from "./commands/install.js";
import { listCommand } from "./commands/list.js";
import { searchCommand } from "./commands/search.js";
import { uninstallCommand } from "./commands/uninstall.js";
import { updateCommand } from "./commands/update.js";

type CommandHandler = (args: string[]) => Promise<number>;

const COMMANDS: Record<string, CommandHandler> = {
  install: installCommand,
  update: updateCommand,
  list: listCommand,
  search: searchCommand,
  uninstall: uninstallCommand,
};

export async function main(args = process.argv.slice(2)): Promise<void> {
  const code = await run(args);
  if (code !== 0) {
    process.exitCode = code;
  }
}

export async function run(args: string[]): Promise<number> {
  const [command, ...rest] = args;

  if (!command || command === "-h" || command === "--help") {
    printHelp();
    return 0;
  }

  if (command === "-v" || command === "--version") {
    console.log(await packageVersion());
    return 0;
  }

  const handler = COMMANDS[command];
  if (!handler) {
    console.error(`Unknown command: ${command}`);
    printHelp();
    return 1;
  }

  return handler(rest);
}

function printHelp(): void {
  console.log(`Printing Press CLI installer

Usage:
  printing-press <command> [options]

Commands:
  install <name|bundle>...  Install one or more Printing Press CLIs (and skills)
  update [name]             Refresh one installed CLI, or all installed CLIs
  list                      List installed Printing Press CLIs
  search <query>            Search the Printing Press catalog
  uninstall <name>          Remove a Printing Press CLI and skill

Bundles:
  starter-pack              espn, flight-goat, movie-goat, recipe-goat

Examples:
  printing-press install starter-pack
  printing-press install espn linear dub
  printing-press install espn --cli-only
  printing-press search sports

Install options:
  --cli-only             Install only the Go binary (skip the focused skill)
  --skill-only           Install only the focused skill (skip the Go binary)
  --agent <agent>        Constrain skill install to a specific agent (repeatable)
  --json                 Emit machine-readable output

Top-level options:
  -h, --help             Show help
  -v, --version          Show version`);
}

async function packageVersion(): Promise<string> {
  let dir = dirname(fileURLToPath(import.meta.url));
  for (let i = 0; i < 5; i++) {
    try {
      const data = await readFile(join(dir, "package.json"), "utf8");
      const parsed = JSON.parse(data) as { version?: string };
      if (parsed.version) {
        return parsed.version;
      }
    } catch {
      // Walk up until we find the package root.
    }
    dir = dirname(dir);
  }
  return "0.0.0";
}
