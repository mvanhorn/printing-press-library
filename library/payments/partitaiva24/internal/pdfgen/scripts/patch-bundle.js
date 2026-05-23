#!/usr/bin/env node
"use strict";

const fs = require("fs");
const path = require("path");

const root = path.resolve(__dirname, "..");
const inputPath = path.join(root, "assets", "bundle.js");
const outputPath = path.join(root, "assets", "bundle.patched.js");
const marker = 'MN=function(e){var t,n,r,i,a,A,o,s=e.locale||"it"';
const injection = ",__p24_pdf_export=(globalThis.__p24_render_pdf=MN)";

function isRegexStart(prev) {
  if (!prev) return true;
  return /[({[=,:;!&|?+\-*~^<>]/.test(prev);
}

function findFunctionEnd(src, start) {
  const braceStart = src.indexOf("{", start);
  if (braceStart < 0) throw new Error("MN function opening brace not found");

  let depth = 0;
  let quote = "";
  let escaped = false;
  let inRegex = false;
  let inClass = false;
  let lineComment = false;
  let blockComment = false;
  let prevSignificant = "";

  for (let i = braceStart; i < src.length; i++) {
    const ch = src[i];
    const next = src[i + 1] || "";

    if (lineComment) {
      if (ch === "\n" || ch === "\r") lineComment = false;
      continue;
    }
    if (blockComment) {
      if (ch === "*" && next === "/") {
        blockComment = false;
        i++;
      }
      continue;
    }
    if (quote) {
      if (escaped) {
        escaped = false;
      } else if (ch === "\\") {
        escaped = true;
      } else if (ch === quote) {
        quote = "";
      } else if (quote === "`" && ch === "$" && next === "{") {
        depth++;
        i++;
      }
      continue;
    }
    if (inRegex) {
      if (escaped) {
        escaped = false;
      } else if (ch === "\\") {
        escaped = true;
      } else if (ch === "[") {
        inClass = true;
      } else if (ch === "]") {
        inClass = false;
      } else if (ch === "/" && !inClass) {
        inRegex = false;
      }
      continue;
    }

    if (ch === "/" && next === "/") {
      lineComment = true;
      i++;
      continue;
    }
    if (ch === "/" && next === "*") {
      blockComment = true;
      i++;
      continue;
    }
    if (ch === '"' || ch === "'" || ch === "`") {
      quote = ch;
      continue;
    }
    if (ch === "/" && isRegexStart(prevSignificant)) {
      inRegex = true;
      continue;
    }
    if (ch === "{") {
      depth++;
    } else if (ch === "}") {
      depth--;
      if (depth === 0) return i + 1;
    }
    if (!/\s/.test(ch)) prevSignificant = ch;
  }

  throw new Error("MN function end not found");
}

const src = fs.readFileSync(inputPath, "utf8");
if (src.includes("globalThis.__p24_render_pdf")) {
  if (fs.existsSync(outputPath)) process.exit(0);
  fs.writeFileSync(outputPath, src);
  process.exit(0);
}

const start = src.indexOf(marker);
if (start < 0) {
  throw new Error(`MN marker not found: ${marker}`);
}
const end = findFunctionEnd(src, start);
const patched = src.slice(0, end) + injection + src.slice(end);
fs.writeFileSync(outputPath, patched);
