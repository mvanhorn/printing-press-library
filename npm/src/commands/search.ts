import {
  DEFAULT_REGISTRY_URL,
  cliBinaryName,
  fetchRegistry,
  type Registry,
  type RegistryEntry,
} from "../registry.js";
import { renderCatalogEntries } from "../format.js";
import { commandPrefixForInvocation } from "../constants.js";

interface SearchDeps {
  fetchRegistry: (url: string) => Promise<Registry>;
  stdout: (message: string) => void;
  stderr: (message: string) => void;
  commandPrefix: string;
}

export function createSearchCommand(overrides: Partial<SearchDeps> = {}) {
  const deps: SearchDeps = {
    fetchRegistry: (url) => fetchRegistry(url),
    stdout: (message) => console.log(message),
    stderr: (message) => console.error(message),
    commandPrefix: commandPrefixForInvocation(),
    ...overrides,
  };

  return async function searchCommandWithDeps(args: string[]): Promise<number> {
    const parsed = parseSearchArgs(args);
    if ("error" in parsed) {
      deps.stderr(parsed.error);
      deps.stderr(`Usage: ${deps.commandPrefix} search <query> [--json]`);
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

    for (const line of renderCatalogEntries(matches, deps.commandPrefix)) {
      deps.stdout(line);
    }
    return 0;
  };
}

export const searchCommand = createSearchCommand();

const MIN_MEANINGFUL_QUERY_LENGTH = 2;
const IGNORED_SEARCH_TERMS = new Set(["pp", "cli", "pp cli"]);

export function searchRegistry(entries: RegistryEntry[], query: string): RegistryEntry[] {
  const terms = searchTerms(query);
  return entries
    .map((entry) => ({ entry, score: scoreEntry(entry, terms) }))
    .filter((result) => result.score > 0)
    .sort((a, b) => b.score - a.score || a.entry.name.localeCompare(b.entry.name))
    .map((result) => result.entry);
}

function scoreEntry(entry: RegistryEntry, terms: string[]): number {
  const name = normalizeCatalogIdentifier(entry.name);
  const binary = normalizeCatalogIdentifier(cliBinaryName(entry));
  const api = normalizeSearchText(entry.api);
  const category = normalizeSearchText(entry.category);
  const description = normalizeSearchText(entry.description);
  const indexText = normalizeSearchText(entry.search_terms?.join(" ") ?? "");
  if (terms.some((term) => name === term || api === term)) return 100;
  if (matchesAnyTerm(name, terms)) return 80;
  if (matchesAnyTerm(binary, terms)) return 80;
  if (matchesAnyTerm(api, terms)) return 70;
  if (matchesAnyTerm(category, terms)) return 50;
  if (matchesAnyTerm(description, terms)) return 30;
  if (matchesAnyTerm(indexText, terms)) return 25;
  return 0;
}

function matchesAnyTerm(value: string, terms: string[]): boolean {
  return terms.some((term) => value.includes(term));
}

function searchTerms(query: string): string[] {
  const normalized = normalizeSearchText(query);
  const terms = new Set<string>();
  addSearchTerm(terms, normalized);
  return [...terms];
}

function isMeaningfulSearchTerm(value: string): boolean {
  const tokens = value.split(/\s+/).filter(Boolean);
  return tokens.some((token) => token.length >= MIN_MEANINGFUL_QUERY_LENGTH) && !IGNORED_SEARCH_TERMS.has(value);
}

function addSearchTerm(terms: Set<string>, term: string): void {
  if (!isMeaningfulSearchTerm(term) || terms.has(term)) {
    return;
  }

  terms.add(term);

  const identifier = stripCommonBinarySuffix(term);
  if (identifier !== term) {
    addSearchTerm(terms, identifier);
  }

  const singular = singularizeTerm(term);
  if (singular !== term) {
    addSearchTerm(terms, singular);
  }
}

function normalizeSearchText(value: string): string {
  return value
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, " ")
    .trim()
    .replace(/\s+/g, " ");
}

function normalizeCatalogIdentifier(value: string): string {
  return stripCommonBinarySuffix(normalizeSearchText(value));
}

function stripCommonBinarySuffix(value: string): string {
  const suffix = " pp cli";
  if (!value.endsWith(suffix)) {
    return value;
  }
  const stripped = value.slice(0, -suffix.length).trim();
  return stripped || value;
}

function singularizeToken(token: string): string {
  return token.length > 3 && token.endsWith("s") && !token.endsWith("ss")
    ? token.slice(0, -1)
    : token;
}

function singularizeTerm(term: string): string {
  return term
    .split(" ")
    .map((token) => singularizeToken(token))
    .join(" ");
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
