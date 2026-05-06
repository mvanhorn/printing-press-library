export const DEFAULT_REGISTRY_URL =
  "https://raw.githubusercontent.com/mvanhorn/printing-press-library/main/registry.json";

export interface MCPBlock {
  binary: string;
  transports: string[];
  tool_count: number;
  public_tool_count?: number;
  auth_type: string;
  env_vars: string[];
  mcp_ready?: string;
}

export interface RegistryEntry {
  name: string;
  category: string;
  api: string;
  description: string;
  path: string;
  mcp?: MCPBlock;
}

export interface Registry {
  schema_version: number;
  entries: RegistryEntry[];
}

type RegistryFetch = (url: string, init?: RequestInit) => Promise<Response>;

export function parseRegistry(value: unknown): Registry {
  if (!isRecord(value)) {
    throw new Error("registry payload must be an object");
  }
  if (value.schema_version !== 2) {
    throw new Error(`unsupported registry schema_version: ${String(value.schema_version)}`);
  }
  if (!Array.isArray(value.entries)) {
    throw new Error("registry entries must be an array");
  }

  return {
    schema_version: 2,
    entries: value.entries.map(parseRegistryEntry),
  };
}

export async function fetchRegistry(
  url = DEFAULT_REGISTRY_URL,
  fetchImpl: RegistryFetch = fetch,
): Promise<Registry> {
  const response = await fetchImpl(url, githubRequestInit(url));
  if (!response.ok) {
    const authHint =
      response.status === 404 || response.status === 401
        ? " If the catalog repository is private, set GITHUB_TOKEN or GH_TOKEN."
        : "";
    throw new Error(`failed to fetch registry: HTTP ${response.status}.${authHint}`);
  }
  return parseRegistry(await response.json());
}

export async function fetchGoModulePath(
  entryPath: string,
  registryUrl = DEFAULT_REGISTRY_URL,
  fetchImpl: RegistryFetch = fetch,
): Promise<string | null> {
  const goModUrl = goModUrlForRegistryEntry(registryUrl, entryPath);
  if (!goModUrl) {
    return null;
  }

  const response = await fetchImpl(goModUrl, githubRequestInit(goModUrl));
  if (!response.ok) {
    return null;
  }

  const goMod = await response.text();
  return parseGoModulePath(goMod);
}

export function lookupByName(registry: Registry, name: string): RegistryEntry | null {
  const normalized = normalizeName(name);
  return (
    registry.entries.find((entry) => {
      const entryName = normalizeName(entry.name);
      return entryName === normalized || normalizeName(entry.api) === normalized;
    }) ?? null
  );
}

export function cliSkillName(entry: RegistryEntry): string {
  return `pp-${entry.name.replace(/-pp-cli$/, "")}`;
}

export function cliBinaryName(entry: RegistryEntry): string {
  return entry.name.endsWith("-pp-cli") ? entry.name : `${entry.name}-pp-cli`;
}

function parseRegistryEntry(value: unknown): RegistryEntry {
  if (!isRecord(value)) {
    throw new Error("registry entry must be an object");
  }

  const entry = {
    name: requiredString(value, "name"),
    category: requiredString(value, "category"),
    api: requiredString(value, "api"),
    description: requiredString(value, "description"),
    path: requiredString(value, "path"),
  };

  return isRecord(value.mcp)
    ? {
        ...entry,
        mcp: {
          binary: requiredString(value.mcp, "binary"),
          transports: requiredStringArray(value.mcp, "transports"),
          tool_count: requiredNumber(value.mcp, "tool_count"),
          public_tool_count:
            typeof value.mcp.public_tool_count === "number" ? value.mcp.public_tool_count : undefined,
          auth_type: requiredString(value.mcp, "auth_type"),
          env_vars: Array.isArray(value.mcp.env_vars) ? value.mcp.env_vars.map(String) : [],
          mcp_ready: typeof value.mcp.mcp_ready === "string" ? value.mcp.mcp_ready : undefined,
        },
      }
    : entry;
}

function normalizeName(value: string): string {
  return value.toLowerCase().replace(/^pp-/, "").replace(/-pp-cli$/, "").replace(/[^a-z0-9]+/g, "-");
}

function requiredString(value: Record<string, unknown>, key: string): string {
  if (typeof value[key] !== "string" || value[key].trim() === "") {
    throw new Error(`registry entry missing string field: ${key}`);
  }
  return value[key];
}

function requiredNumber(value: Record<string, unknown>, key: string): number {
  if (typeof value[key] !== "number") {
    throw new Error(`registry entry missing number field: ${key}`);
  }
  return value[key];
}

function requiredStringArray(value: Record<string, unknown>, key: string): string[] {
  const raw = value[key];
  if (!Array.isArray(raw) || raw.length === 0) {
    throw new Error(`registry entry missing non-empty string array field: ${key}`);
  }
  const out: string[] = [];
  for (const item of raw) {
    if (typeof item !== "string" || item.trim() === "") {
      throw new Error(`registry entry has non-string value in array field: ${key}`);
    }
    out.push(item);
  }
  return out;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

export function parseGoModulePath(goMod: string): string | null {
  for (const line of goMod.split(/\r?\n/)) {
    const match = line.match(/^module\s+(\S+)/);
    if (match) {
      return match[1]!;
    }
  }
  return null;
}

function goModUrlForRegistryEntry(registryUrl: string, entryPath: string): string | null {
  let parsed: URL;
  try {
    parsed = new URL(registryUrl);
  } catch {
    return null;
  }

  if (parsed.hostname !== "raw.githubusercontent.com") {
    return null;
  }

  const parts = parsed.pathname.split("/").filter(Boolean);
  if (parts.length < 4 || parts.at(-1) !== "registry.json") {
    return null;
  }

  parsed.pathname = `/${parts.slice(0, -1).join("/")}/${entryPath}/go.mod`;
  return parsed.toString();
}

function githubRequestInit(url: string): RequestInit | undefined {
  const token = process.env.GITHUB_TOKEN || process.env.GH_TOKEN;
  if (!token || !isTrustedGitHubUrl(url)) {
    return undefined;
  }
  return {
    headers: {
      Authorization: `Bearer ${token}`,
      "X-GitHub-Api-Version": "2022-11-28",
    },
  };
}

function isTrustedGitHubUrl(url: string): boolean {
  try {
    const parsed = new URL(url);
    return parsed.hostname === "raw.githubusercontent.com" || parsed.hostname === "api.github.com";
  } catch {
    return false;
  }
}
