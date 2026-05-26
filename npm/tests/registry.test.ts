import test from "node:test";
import assert from "node:assert/strict";
import {
  cliBinaryName,
  cliSkillName,
  fetchGoModulePath,
  fetchRegistry,
  lookupByName,
  parseGoModulePath,
  parseRegistry,
} from "../src/registry.js";

test("parseRegistry validates and returns registry entries", () => {
  const registry = parseRegistry({
    schema_version: 2,
    entries: [
      {
        name: "espn",
        category: "sports",
        api: "ESPN",
        description: "Sports scores",
        search_terms: ["NBA scores", "sports standings"],
        path: "library/sports/espn",
      },
    ],
  });

  assert.equal(registry.entries.length, 1);
  assert.equal(registry.entries[0]?.name, "espn");
  assert.deepEqual(registry.entries[0]?.search_terms, ["NBA scores", "sports standings"]);
});

test("parseRegistry treats null optional search_terms as absent", () => {
  const registry = parseRegistry({
    schema_version: 2,
    entries: [
      {
        name: "espn",
        category: "sports",
        api: "ESPN",
        description: "Sports scores",
        search_terms: null,
        path: "library/sports/espn",
      },
    ],
  });

  assert.equal(registry.entries.length, 1);
  assert.equal(registry.entries[0]?.search_terms, undefined);
});

test("lookupByName matches normalized CLI and API names", () => {
  const registry = parseRegistry({
    schema_version: 2,
    entries: [
      {
        name: "yahoo-finance-pp-cli",
        category: "finance",
        api: "Yahoo Finance",
        description: "Market data",
        path: "library/finance/yahoo-finance",
      },
    ],
  });

  assert.equal(lookupByName(registry, "yahoo-finance")?.path, "library/finance/yahoo-finance");
  assert.equal(lookupByName(registry, "yahoo-finance-pp-cli")?.path, "library/finance/yahoo-finance");
  assert.equal(lookupByName(registry, "pp-yahoo-finance")?.path, "library/finance/yahoo-finance");
  assert.equal(lookupByName(registry, "Yahoo Finance")?.path, "library/finance/yahoo-finance");
  assert.equal(lookupByName(registry, "missing"), null);
});

test("cliSkillName preserves pp- naming convention", () => {
  const registry = parseRegistry({
    schema_version: 2,
    entries: [
      {
        name: "dominos-pp-cli",
        category: "commerce",
        api: "Dominos",
        description: "Pizza ordering",
        path: "library/commerce/dominos",
      },
    ],
  });

  assert.equal(cliSkillName(registry.entries[0]!), "pp-dominos");
  assert.equal(cliBinaryName(registry.entries[0]!), "dominos-pp-cli");
});

test("parseRegistry rejects unsupported schema versions", () => {
  assert.throws(() => parseRegistry({ schema_version: 1, entries: [] }), /unsupported registry/);
  assert.throws(() => parseRegistry({ schema_version: 3, entries: [] }), /unsupported registry/);
});

test("parseRegistry parses transports as a non-empty string array", () => {
  const registry = parseRegistry({
    schema_version: 2,
    entries: [
      {
        name: "ahrefs",
        category: "marketing",
        api: "Ahrefs",
        description: "Backlinks and SEO",
        path: "library/marketing/ahrefs",
        mcp: {
          binary: "ahrefs-pp-mcp",
          transports: ["stdio", "http"],
          tool_count: 29,
          public_tool_count: 2,
          auth_type: "api_key",
          env_vars: ["AHREFS_API_KEY"],
        },
      },
    ],
  });

  assert.deepEqual(registry.entries[0]?.mcp?.transports, ["stdio", "http"]);
});

test("parseRegistry skips entries with empty or missing transports and warns", () => {
  const baseEntry = {
    name: "demo",
    category: "demo",
    api: "Demo",
    description: "Demo",
    path: "library/demo/demo",
  };
  const baseMcp = {
    binary: "demo-pp-mcp",
    tool_count: 1,
    auth_type: "none",
    env_vars: [],
  };

  const warningsEmpty: string[] = [];
  const r1 = parseRegistry(
    {
      schema_version: 2,
      entries: [{ ...baseEntry, mcp: { ...baseMcp, transports: [] } }],
    },
    (m) => warningsEmpty.push(m),
  );
  assert.equal(r1.entries.length, 0, "malformed entry must be skipped, not silently included");
  assert.ok(
    warningsEmpty.some((w) => w.includes("demo") && w.includes("transports")),
    `expected a per-entry skip warning naming the entry and field; got: ${JSON.stringify(warningsEmpty)}`,
  );
  assert.ok(
    warningsEmpty.some((w) => w.includes("skipped 1 malformed registry entry")),
    "expected a final summary warning",
  );

  const warningsNonString: string[] = [];
  const r2 = parseRegistry(
    {
      schema_version: 2,
      entries: [{ ...baseEntry, mcp: { ...baseMcp, transports: ["stdio", 7] } }],
    },
    (m) => warningsNonString.push(m),
  );
  assert.equal(r2.entries.length, 0);
  assert.ok(warningsNonString.some((w) => w.includes("transports")));
});

test("parseRegistry keeps valid entries when one is malformed (lawhub-shape regression)", () => {
  const warnings: string[] = [];
  const registry = parseRegistry(
    {
      schema_version: 2,
      entries: [
        {
          name: "ok",
          category: "cat",
          api: "OK",
          description: "Has a description.",
          path: "library/cat/ok",
        },
        // The lawhub case: description is empty. Old behavior threw; new
        // behavior must skip this entry and keep the valid ones loadable.
        {
          name: "lawhub",
          category: "education",
          api: "LawHub",
          description: "",
          path: "library/education/lawhub",
        },
        {
          name: "ok2",
          category: "cat",
          api: "OK2",
          description: "Also fine.",
          path: "library/cat/ok2",
        },
      ],
    },
    (m) => warnings.push(m),
  );

  assert.equal(registry.entries.length, 2, "valid entries must survive a malformed sibling");
  assert.deepEqual(
    registry.entries.map((e) => e.name).sort(),
    ["ok", "ok2"],
  );
  assert.ok(
    warnings.some((w) => w.includes("lawhub") && w.includes("description")),
    "expected per-entry warning naming the offending slug and field",
  );
});

test("parseRegistry uses fallback identifier when malformed entry has no name", () => {
  const warnings: string[] = [];
  const registry = parseRegistry(
    {
      schema_version: 2,
      entries: [
        {
          // No name field at all — fallback to (unnamed at index N).
          category: "cat",
          api: "X",
          description: "Has desc.",
          path: "library/cat/x",
        },
      ],
    },
    (m) => warnings.push(m),
  );

  assert.equal(registry.entries.length, 0);
  assert.ok(
    warnings.some((w) => w.includes("(unnamed at index 0)")),
    `expected unnamed-at-index fallback; got: ${JSON.stringify(warnings)}`,
  );
});

test("parseRegistry preserves registry-level errors as throws (not per-entry)", () => {
  // Wrong shape at the registry level is not recoverable per-entry.
  assert.throws(() => parseRegistry({ schema_version: 1, entries: [] }), /unsupported registry/);
  assert.throws(() => parseRegistry({ schema_version: 2, entries: "not-an-array" }), /must be an array/);
  assert.throws(() => parseRegistry("not-an-object"), /must be an object/);
});

test("fetchRegistry sends GitHub token when available", async () => {
  const previous = process.env.GITHUB_TOKEN;
  process.env.GITHUB_TOKEN = "test-token";
  let authHeader: string | null = null;
  try {
    await fetchRegistry(
      "https://raw.githubusercontent.com/mvanhorn/printing-press-library/main/registry.json",
      async (_url, init) => {
        authHeader = new Headers(init?.headers).get("authorization");
        return new Response(
          JSON.stringify({
            schema_version: 2,
            entries: [],
          }),
          { status: 200 },
        );
      },
    );
  } finally {
    if (previous === undefined) {
      delete process.env.GITHUB_TOKEN;
    } else {
      process.env.GITHUB_TOKEN = previous;
    }
  }

  assert.equal(authHeader, "Bearer test-token");
});

test("fetchRegistry does not send GitHub token to custom registry hosts", async () => {
  const previous = process.env.GITHUB_TOKEN;
  process.env.GITHUB_TOKEN = "test-token";
  let authHeader: string | null = null;
  try {
    await fetchRegistry("https://registry.example.test/registry.json", async (_url, init) => {
      authHeader = new Headers(init?.headers).get("authorization");
      return new Response(JSON.stringify({ schema_version: 2, entries: [] }), { status: 200 });
    });
  } finally {
    if (previous === undefined) {
      delete process.env.GITHUB_TOKEN;
    } else {
      process.env.GITHUB_TOKEN = previous;
    }
  }

  assert.equal(authHeader, null);
});

test("fetchGoModulePath reads go.mod next to a registry entry", async () => {
  let requestedUrl = "";
  const modulePath = await fetchGoModulePath(
    "library/sales-and-crm/hubspot",
    "https://raw.githubusercontent.com/mvanhorn/printing-press-library/main/registry.json",
    async (url) => {
      requestedUrl = url;
      return new Response(
        "module github.com/mvanhorn/printing-press-library/library/sales-and-crm/hubspot-pp-cli\n",
        { status: 200 },
      );
    },
  );

  assert.equal(
    requestedUrl,
    "https://raw.githubusercontent.com/mvanhorn/printing-press-library/main/library/sales-and-crm/hubspot/go.mod",
  );
  assert.equal(
    modulePath,
    "github.com/mvanhorn/printing-press-library/library/sales-and-crm/hubspot-pp-cli",
  );
});

test("parseGoModulePath returns null when no module declaration exists", () => {
  assert.equal(parseGoModulePath("go 1.23\n"), null);
});
