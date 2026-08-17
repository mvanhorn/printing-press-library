#!/usr/bin/env node
/**
 * framer-bridge.mjs — Node.js bridge between Go CLI and Framer Server API.
 *
 * Usage: node framer-bridge.mjs <command> [args-as-json]
 *
 * Env vars required:
 *   FRAMER_API_KEY       — per-project API key
 *   FRAMER_PROJECT_URL   — project URL (https://framer.com/projects/<id>)
 *
 * Commands:
 *   project-info          — get project name and ID
 *   collections-list      — list all CMS collections
 *   collection-get <id>   — get collection details + fields
 *   items-list <collectionId> — list all items in a collection
 *   item-get <itemId>     — get a single CMS item
 *   items-upsert <json>   — create/update items (json: {collectionId, items[]})
 *   items-remove <json>   — remove items (json: {ids[]})
 *   code-list             — list all code files
 *   code-get <id>         — get code file content
 *   code-create <json>    — create code file (json: {name, content})
 *   assets-upload <json>  — upload image files (json: {paths:[...]}) -> [{name,id,url}]
 *   pages-list            — list all pages
 *   page-create <json>    — create page (json: {name, type})
 *   styles-colors-list    — list color styles
 *   styles-colors-create <json> — create color style (json: {name, value})
 *   styles-text-list      — list text styles
 *   fonts-list            — list fonts
 *   publish-preview       — create preview deployment
 *   deploy <deploymentId> — deploy to production
 *   changes-list          — list changed paths
 *   changes-contributors  — list change contributors
 *   redirects-list        — list redirects
 *   redirects-add <json>  — add redirects
 *   locales-list          — list locales
 *   nodes-get <id>        — get node by ID
 *   nodes-children <id>   — get children of node
 *   nodes-find <json>     — find nodes (json: {type?, name?, attribute?})
 *   canvas-root           — get canvas root node
 *   custom-code-get       — get custom code
 *   sync-all              — fetch all resources for local store
 */

import { connect } from "framer-api";
import { readFileSync } from "node:fs";
import { basename, extname } from "node:path";

const PROJECT_URL = process.env.FRAMER_PROJECT_URL;
const API_KEY = process.env.FRAMER_API_KEY;

if (!PROJECT_URL) {
  console.error(JSON.stringify({ error: "FRAMER_PROJECT_URL not set" }));
  process.exit(1);
}
if (!API_KEY) {
  console.error(JSON.stringify({ error: "FRAMER_API_KEY not set" }));
  process.exit(1);
}

const command = process.argv[2];
const arg = process.argv[3];

async function run() {
  const framer = await connect(PROJECT_URL, API_KEY);

  try {
    let result;

    switch (command) {
      // === Project ===
      case "project-info": {
        const info = await framer.getProjectInfo();
        result = info;
        break;
      }

      // === CMS Collections ===
      case "collections-list": {
        const collections = await framer.getCollections();
        const out = [];
        for (const c of collections) {
          const fields = await c.getFields();
          const items = await c.getItems();
          out.push({
            id: c.id,
            name: c.name || c.id,
            fieldCount: fields.length,
            itemCount: items.length,
            fields: fields.map((f) => ({
              id: f.id,
              name: f.name,
              type: f.type,
            })),
          });
        }
        result = out;
        break;
      }

      case "collection-get": {
        if (!arg) throw new Error("collection ID required");
        const collections = await framer.getCollections();
        const c = collections.find((x) => x.id === arg);
        if (!c) throw new Error(`collection ${arg} not found`);
        const fields = await c.getFields();
        const items = await c.getItems();
        result = {
          id: c.id,
          name: c.name || c.id,
          fields: fields.map((f) => ({
            id: f.id,
            name: f.name,
            type: f.type,
          })),
          items: items.map((i) => ({
            id: i.id,
            slug: i.slug,
            draft: i.draft,
            fieldData: i.fieldData,
          })),
        };
        break;
      }

      // === CMS Items ===
      case "items-list": {
        if (!arg) throw new Error("collection ID required");
        const collections = await framer.getCollections();
        const c = collections.find((x) => x.id === arg);
        if (!c) throw new Error(`collection ${arg} not found`);
        const items = await c.getItems();
        result = items.map((i) => ({
          id: i.id,
          slug: i.slug,
          draft: i.draft,
          fieldData: i.fieldData,
        }));
        break;
      }

      case "items-upsert": {
        if (!arg) throw new Error("JSON argument required: {collectionId, items: [{slug, fieldData}]}");
        const params = JSON.parse(arg);
        if (!params.collectionId) throw new Error("collectionId required");
        if (!Array.isArray(params.items) || params.items.length === 0) throw new Error("items array required");
        const collections = await framer.getCollections();
        const c = collections.find((x) => x.id === params.collectionId);
        if (!c) throw new Error(`collection ${params.collectionId} not found`);
        await c.addItems(params.items);
        result = { upserted: true, count: params.items.length };
        break;
      }

      case "items-remove": {
        if (!arg) throw new Error("JSON argument required: {collectionId, ids: []}");
        const params = JSON.parse(arg);
        if (!params.collectionId) throw new Error("collectionId required");
        if (!Array.isArray(params.ids) || params.ids.length === 0) throw new Error("ids array required");
        const collections = await framer.getCollections();
        const c = collections.find((x) => x.id === params.collectionId);
        if (!c) throw new Error(`collection ${params.collectionId} not found`);
        await c.removeItems(params.ids);
        result = { removed: true, count: params.ids.length };
        break;
      }

      // === Code Files ===
      case "code-list": {
        const files = await framer.getCodeFiles();
        result = await Promise.all(
          files.map(async (f) => ({
            id: f.id,
            name: f.name,
            path: f.path,
          }))
        );
        break;
      }

      case "code-get": {
        if (!arg) throw new Error("code file ID required");
        const file = await framer.getCodeFile(arg);
        if (!file) throw new Error(`code file ${arg} not found`);
        const content = await file.content;
        result = {
          id: file.id,
          name: file.name,
          path: file.path,
          content: content,
        };
        break;
      }

      case "code-create": {
        const params = JSON.parse(arg);
        const file = await framer.createCodeFile(params.name, params.content || "");
        result = { id: file.id, name: file.name, path: file.path };
        break;
      }

      case "code-update": {
        const params = JSON.parse(arg);
        if (!params.id) throw new Error("code file ID required");
        const file = await framer.getCodeFile(params.id);
        if (!file) throw new Error(`code file ${params.id} not found`);
        await file.setFileContent(params.content || "");
        result = { id: file.id, name: file.name, updated: true };
        break;
      }

      // === Assets ===
      case "assets-upload": {
        const params = JSON.parse(arg);
        const paths = params.paths || (params.path ? [params.path] : []);
        if (!paths.length) throw new Error("assets-upload requires {paths:[...]}");
        const MIME = {
          ".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg",
          ".webp": "image/webp", ".gif": "image/gif", ".svg": "image/svg+xml",
        };
        const inputs = paths.map((p) => {
          const mimeType = MIME[extname(p).toLowerCase()];
          if (!mimeType) throw new Error(`unsupported image type: ${p}`);
          return {
            name: basename(p),
            image: { bytes: new Uint8Array(readFileSync(p)), mimeType },
          };
        });
        const assets = await framer.uploadImages(inputs);
        result = assets.map((a, i) => ({
          name: inputs[i].name, id: a.id, url: a.url,
        }));
        break;
      }

      // === Pages ===
      case "pages-list": {
        const root = await framer.getCanvasRoot();
        const children = await root.getChildren();
        result = await Promise.all(
          children.map(async (n) => ({
            id: n.id,
            name: n.name || n.id,
            type: n.type || "unknown",
          }))
        );
        break;
      }

      // === Styles ===
      case "styles-colors-list": {
        const styles = await framer.getColorStyles();
        result = styles.map((s) => ({
          id: s.id,
          name: s.name,
          light: s.light,
          dark: s.dark,
        }));
        break;
      }

      case "styles-colors-create": {
        const params = JSON.parse(arg);
        const style = await framer.createColorStyle({
          name: params.name,
          light: params.value,
          dark: params.dark || params.value,
        });
        result = { id: style.id, name: style.name };
        break;
      }

      case "styles-text-list": {
        const styles = await framer.getTextStyles();
        result = styles.map((s) => ({
          id: s.id,
          name: s.name,
          tag: s.tag,
          font: s.font,
          fontSize: s.fontSize,
          lineHeight: s.lineHeight,
          letterSpacing: s.letterSpacing,
        }));
        break;
      }

      // === Fonts ===
      case "fonts-list": {
        const fonts = await framer.getFonts();
        result = fonts.map((f) => ({
          family: f.family,
          weights: f.weights,
          styles: f.styles,
        }));
        break;
      }

      // === Publishing ===
      case "publish-preview": {
        const deployment = await framer.publish();
        result = deployment;
        break;
      }

      case "deploy": {
        if (!arg) throw new Error("deployment ID required");
        await framer.deploy(arg);
        result = { deployed: true, deploymentId: arg };
        break;
      }

      // === Changes ===
      case "changes-list": {
        const changes = await framer.getChangedPaths();
        result = changes;
        break;
      }

      case "changes-contributors": {
        const contributors = await framer.getChangeContributors();
        result = contributors;
        break;
      }

      // === Redirects ===
      case "redirects-list": {
        const redirects = await framer.getRedirects();
        result = redirects;
        break;
      }

      case "redirects-add": {
        const params = JSON.parse(arg);
        await framer.addRedirects(params);
        result = { added: true, count: params.length };
        break;
      }

      // === Localization ===
      case "locales-list": {
        const locales = await framer.getLocales();
        result = locales.map((l) => ({
          id: l.id,
          code: l.code,
          name: l.name,
          slug: l.slug,
        }));
        break;
      }

      // === Nodes ===
      case "canvas-root": {
        const root = await framer.getCanvasRoot();
        result = {
          id: root.id,
          name: root.name,
          type: root.type,
        };
        break;
      }

      case "nodes-get": {
        if (!arg) throw new Error("node ID required");
        const node = await framer.getNode(arg);
        if (!node) throw new Error(`node ${arg} not found`);
        result = {
          id: node.id,
          name: node.name,
          type: node.type,
        };
        break;
      }

      case "nodes-children": {
        if (!arg) throw new Error("node ID required");
        const node = await framer.getNode(arg);
        if (!node) throw new Error(`node ${arg} not found`);
        const children = await node.getChildren();
        result = children.map((c) => ({
          id: c.id,
          name: c.name,
          type: c.type,
        }));
        break;
      }

      // === Node Mutations ===
      case "nodes-remove": {
        if (!arg) throw new Error("node ID required");
        await framer.removeNode(arg);
        result = { removed: true, id: arg };
        break;
      }

      case "nodes-set-parent": {
        const params = JSON.parse(arg);
        if (!params.id) throw new Error("node id required");
        if (!params.parentId) throw new Error("parentId required");
        await framer.setParent(params.id, params.parentId, params.index ?? undefined);
        result = { moved: true, id: params.id, parentId: params.parentId, index: params.index ?? null };
        break;
      }

      case "nodes-set-attributes": {
        const params = JSON.parse(arg);
        if (!params.id) throw new Error("node id required");
        if (!params.attributes) throw new Error("attributes object required");
        await framer.setAttributes(params.id, params.attributes);
        result = { updated: true, id: params.id };
        break;
      }

      case "nodes-create-frame": {
        const params = JSON.parse(arg);
        const node = await framer.createFrameNode(params);
        result = { id: node.id, name: node.name, type: node.type };
        break;
      }

      case "nodes-clone": {
        if (!arg) throw new Error("node ID required");
        const cloned = await framer.cloneNode(arg);
        result = { id: cloned.id, name: cloned.name, type: cloned.type };
        break;
      }

      case "components-add": {
        const params = JSON.parse(arg);
        // If codeFileId is provided, resolve its insertURL
        let insertURL = params.insertURL || params.url;
        if (!insertURL && params.codeFileId) {
          const file = await framer.getCodeFile(params.codeFileId);
          if (!file) throw new Error(`code file ${params.codeFileId} not found`);
          const exports = file.exports;
          const compExport = exports.find(e => e.type === "component");
          if (!compExport) throw new Error(`code file ${params.codeFileId} has no component export`);
          insertURL = compExport.insertURL;
        }
        if (!insertURL && params.codeFileName) {
          // Resolve by name
          const files = await framer.getCodeFiles();
          const variants = [params.codeFileName];
          if (!params.codeFileName.endsWith(".tsx")) variants.push(params.codeFileName + ".tsx");
          let matched = null;
          for (const v of variants) {
            matched = files.find(f => f.name === v || f.name.toLowerCase() === v.toLowerCase());
            if (matched) break;
          }
          if (!matched) throw new Error(`code file '${params.codeFileName}' not found`);
          const exports = matched.exports;
          const compExport = exports.find(e => e.type === "component");
          if (!compExport) throw new Error(`code file '${params.codeFileName}' has no component export`);
          insertURL = compExport.insertURL;
        }
        if (!insertURL) throw new Error("insertURL, codeFileId, or codeFileName required");
        const instance = await framer.addComponentInstance({ url: insertURL, attributes: params.attributes || {} });
        result = { id: instance.id, name: instance.name, type: instance.type, insertURL };
        break;
      }

      // === Custom Code ===
      case "custom-code-get": {
        const code = await framer.getCustomCode();
        result = code;
        break;
      }

      // === Sync All (for local store population) ===
      case "sync-all": {
        const resources = {};

        // Project info
        const info = await framer.getProjectInfo();
        resources.project = [info];

        // Collections + items
        const collections = await framer.getCollections();
        resources.collections = [];
        resources.items = [];
        for (const c of collections) {
          const fields = await c.getFields();
          const items = await c.getItems();
          resources.collections.push({
            id: c.id,
            name: c.name || c.id,
            fields: fields.map((f) => ({
              id: f.id,
              name: f.name,
              type: f.type,
            })),
            itemCount: items.length,
          });
          for (const item of items) {
            resources.items.push({
              id: item.id,
              slug: item.slug,
              draft: item.draft,
              collectionId: c.id,
              collectionName: c.name || c.id,
              fieldData: item.fieldData,
            });
          }
        }

        // Code files
        const codeFiles = await framer.getCodeFiles();
        resources.codeFiles = await Promise.all(
          codeFiles.map(async (f) => {
            let content = "";
            try {
              content = await f.content;
            } catch (e) {}
            return {
              id: f.id,
              name: f.name,
              path: f.path,
              content: content,
            };
          })
        );

        // Color styles
        const colorStyles = await framer.getColorStyles();
        resources.colorStyles = colorStyles.map((s) => ({
          id: s.id,
          name: s.name,
          light: s.light,
          dark: s.dark,
        }));

        // Text styles
        const textStyles = await framer.getTextStyles();
        resources.textStyles = textStyles.map((s) => ({
          id: s.id,
          name: s.name,
          tag: s.tag,
          font: s.font,
          fontSize: s.fontSize,
        }));

        // Redirects
        try {
          const redirects = await framer.getRedirects();
          resources.redirects = redirects;
        } catch (e) {
          resources.redirects = [];
        }

        // Locales
        try {
          const locales = await framer.getLocales();
          resources.locales = locales.map((l) => ({
            id: l.id,
            code: l.code,
            name: l.name,
            slug: l.slug,
          }));
        } catch (e) {
          resources.locales = [];
        }

        // Pages (top-level canvas children)
        try {
          const root = await framer.getCanvasRoot();
          const children = await root.getChildren();
          resources.pages = children.map((n) => ({
            id: n.id,
            name: n.name || n.id,
            type: n.type || "unknown",
          }));
        } catch (e) {
          resources.pages = [];
        }

        result = resources;
        break;
      }

      default:
        throw new Error(`unknown command: ${command}`);
    }

    console.log(JSON.stringify(result, null, 2));
  } finally {
    await framer.disconnect();
  }
}

run().catch((err) => {
  console.error(JSON.stringify({ error: err.message, stack: err.stack }));
  process.exit(1);
});
