import { DEFAULT_REGISTRY_URL, fetchRegistry, type Registry, type RegistryEntry } from "../registry.js";
import { renderCatalogEntries } from "../format.js";

interface SearchDeps {
  fetchRegistry: (url: string) => Promise<Registry>;
  stdout: (message: string) => void;
  stderr: (message: string) => void;
}

export function createSearchCommand(overrides: Partial<SearchDeps> = {}) {
  const deps: SearchDeps = {
    fetchRegistry: (url) => fetchRegistry(url),
    stdout: (message) => console.log(message),
    stderr: (message) => console.error(message),
    ...overrides,
  };

  return async function searchCommandWithDeps(args: string[]): Promise<number> {
    const parsed = parseSearchArgs(args);
    if ("error" in parsed) {
      deps.stderr(parsed.error);
      deps.stderr("Usage: printing-press-library search <query> [--json]");
      return 1;
    }

    const registry = await deps.fetchRegistry(parsed.registryUrl);
    const matches = searchRegistry(registry.entries, parsed.query);

    if (parsed.json) {
      deps.stdout(JSON.stringify(matches, null, 2));
      return 0;
    }

    if (matches.length === 0) {
      deps.stdout(`No matches for "${parsed.query}".`);
      return 0;
    }

    for (const line of renderCatalogEntries(matches)) {
      deps.stdout(line);
    }
    return 0;
  };
}

export const searchCommand = createSearchCommand();

export function searchRegistry(entries: RegistryEntry[], query: string): RegistryEntry[] {
  const q = query.toLowerCase();
  return entries
    .map((entry) => ({ entry, score: scoreEntry(entry, q) }))
    .filter((result) => result.score > 0)
    .sort((a, b) => b.score - a.score || a.entry.name.localeCompare(b.entry.name))
    .map((result) => result.entry);
}

function scoreEntry(entry: RegistryEntry, query: string): number {
  const name = entry.name.toLowerCase();
  const api = entry.api.toLowerCase();
  const category = entry.category.toLowerCase();
  const description = entry.description.toLowerCase();
  if (name === query || api === query) return 100;
  if (name.includes(query)) return 80;
  if (api.includes(query)) return 70;
  if (category.includes(query)) return 50;
  if (description.includes(query)) return 30;
  return 0;
}

function parseSearchArgs(args: string[]):
  | { query: string; json: boolean; registryUrl: string }
  | { error: string } {
  const queryParts: string[] = [];
  let json = false;
  let registryUrl = DEFAULT_REGISTRY_URL;

  for (let i = 0; i < args.length; i++) {
    const arg = args[i]!;
    if (arg === "--json") {
      json = true;
    } else if (arg === "--registry-url") {
      const value = args[++i];
      if (!value) {
        return { error: "Missing value for --registry-url" };
      }
      registryUrl = value;
    } else if (arg.startsWith("-")) {
      return { error: `Unknown search option: ${arg}` };
    } else {
      queryParts.push(arg);
    }
  }

  const query = queryParts.join(" ").trim();
  return query ? { query, json, registryUrl } : { error: "Missing search query" };
}
