#!/usr/bin/env node
/**
 * AP-033: Bidirectional OpenAPI ↔ Gin route contract gate.
 *
 * Extracts METHOD+path from:
 *   - backend/internal/routes/router.go (static parse of Gin registrations)
 *   - docs/openapi.yaml (paths + methods)
 *
 * Compares /api/v1 routes both ways and fails on mismatch.
 * Usage: node scripts/openapi-contract-gate.mjs
 * Exit 0 on match, 1 on mismatch.
 */
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const ROOT = path.resolve(__dirname, "..");
const ROUTER = path.join(ROOT, "backend/internal/routes/router.go");
const OPENAPI = path.join(ROOT, "docs/openapi.yaml");
const API_PREFIX = "/api/v1";

const HTTP_METHODS = new Set([
  "GET",
  "POST",
  "PUT",
  "PATCH",
  "DELETE",
  "HEAD",
  "OPTIONS",
]);

function die(msg) {
  console.error(`openapi-contract-gate: ${msg}`);
  process.exit(1);
}

function read(file) {
  if (!fs.existsSync(file)) die(`missing file: ${file}`);
  return fs.readFileSync(file, "utf8");
}

/** Normalize path params: :id / {id} → {id}; collapse //; trim trailing slash. */
function normalizePath(p) {
  let s = String(p || "").trim();
  if (!s.startsWith("/")) s = `/${s}`;
  s = s.replace(/:([A-Za-z_][A-Za-z0-9_]*)/g, "{$1}");
  s = s.replace(/\{([A-Za-z_][A-Za-z0-9_]*)\}/g, (_, name) => `{${name}}`);
  s = s.replace(/\/{2,}/g, "/");
  if (s.length > 1 && s.endsWith("/")) s = s.slice(0, -1);
  return s;
}

function key(method, p) {
  return `${method.toUpperCase()} ${normalizePath(p)}`;
}

/**
 * Static parse of router.go Gin registrations.
 * Tracks group prefixes via r.Group / x.Group assignments.
 */
function extractGinRoutes(src) {
  const groups = new Map();
  groups.set("r", "");

  const groupRe = /(\w+)\s*:?=\s*(\w+)\.Group\(\s*"([^"]*)"\s*\)/g;
  let m;
  while ((m = groupRe.exec(src)) !== null) {
    const [, lhs, parent, segment] = m;
    const parentPrefix = groups.has(parent) ? groups.get(parent) : "";
    let prefix;
    if (segment === "") {
      prefix = parentPrefix || "";
    } else {
      prefix = normalizePath(
        `${parentPrefix}${segment.startsWith("/") ? segment : `/${segment}`}`,
      );
    }
    groups.set(lhs, prefix === "/" ? "" : prefix);
  }

  const routes = new Map();
  const routeRe =
    /(\w+)\.(GET|POST|PUT|PATCH|DELETE|HEAD|OPTIONS)\(\s*"([^"]+)"/g;
  const full = src;
  while ((m = routeRe.exec(full)) !== null) {
    const [, recv, method, rawPath] = m;
    const before = full.slice(0, m.index);
    const lineNo = before.split("\n").length;
    const base = groups.has(recv) ? groups.get(recv) : "";
    let fullPath;
    if (base) {
      fullPath = `${base}${rawPath.startsWith("/") ? rawPath : `/${rawPath}`}`;
    } else {
      fullPath = rawPath;
    }
    fullPath = normalizePath(fullPath);
    const k = key(method, fullPath);
    routes.set(k, {
      method: method.toUpperCase(),
      path: fullPath,
      line: lineNo,
      source: "gin",
    });
  }
  return routes;
}

/**
 * Minimal YAML path extractor for OpenAPI paths: section.
 */
function extractOpenAPIRoutes(yamlText) {
  const routes = new Map();
  const lines = yamlText.split(/\r?\n/);
  let inPaths = false;
  let currentPath = null;

  for (let i = 0; i < lines.length; i++) {
    const line = lines[i];
    const trimmed = line.trimEnd();
    if (!inPaths) {
      if (/^paths:\s*$/.test(trimmed)) {
        inPaths = true;
      }
      continue;
    }
    if (
      trimmed !== "" &&
      !trimmed.startsWith("#") &&
      !/^\s/.test(line) &&
      !/^paths:/.test(trimmed)
    ) {
      break;
    }
    if (trimmed === "" || trimmed.trimStart().startsWith("#")) continue;

    const pathMatch = line.match(/^(\s*)(\/[^:]*):\s*$/);
    if (pathMatch) {
      currentPath = normalizePath(pathMatch[2].trim());
      continue;
    }

    if (!currentPath) continue;

    const methodMatch = line.match(
      /^(\s*)(get|post|put|patch|delete|head|options):\s*$/i,
    );
    if (methodMatch) {
      const indent = methodMatch[1].length;
      if (indent >= 2) {
        const method = methodMatch[2].toUpperCase();
        if (HTTP_METHODS.has(method)) {
          const k = key(method, currentPath);
          routes.set(k, {
            method,
            path: currentPath,
            line: i + 1,
            source: "openapi",
          });
        }
      }
    }
  }
  return routes;
}

function filterApiV1(map) {
  const out = new Map();
  for (const [k, v] of map) {
    if (v.path === API_PREFIX || v.path.startsWith(`${API_PREFIX}/`)) {
      out.set(k, v);
    }
  }
  return out;
}

function main() {
  const ginAll = extractGinRoutes(read(ROUTER));
  const oasAll = extractOpenAPIRoutes(read(OPENAPI));

  const gin = filterApiV1(ginAll);
  const oas = filterApiV1(oasAll);

  const onlyGin = [];
  const onlyOas = [];

  for (const k of gin.keys()) {
    if (!oas.has(k)) onlyGin.push(gin.get(k));
  }
  for (const k of oas.keys()) {
    if (!gin.has(k)) onlyOas.push(oas.get(k));
  }

  onlyGin.sort(
    (a, b) => a.path.localeCompare(b.path) || a.method.localeCompare(b.method),
  );
  onlyOas.sort(
    (a, b) => a.path.localeCompare(b.path) || a.method.localeCompare(b.method),
  );

  console.log(`AP-033 OpenAPI contract gate`);
  console.log(`  router.go /api/v1 operations: ${gin.size}`);
  console.log(`  openapi.yaml /api/v1 operations: ${oas.size}`);

  if (onlyGin.length === 0 && onlyOas.length === 0) {
    console.log(`  OK: bidirectional match for ${API_PREFIX}`);
    const keys = [...gin.keys()].sort();
    for (const k of keys) console.log(`    ✓ ${k}`);
    process.exit(0);
  }

  if (onlyGin.length) {
    console.error(`\n  Runtime routes missing from OpenAPI (${onlyGin.length}):`);
    for (const r of onlyGin) {
      console.error(`    - ${r.method} ${r.path}  (router.go:${r.line})`);
    }
  }
  if (onlyOas.length) {
    console.error(`\n  OpenAPI paths missing from runtime (${onlyOas.length}):`);
    for (const r of onlyOas) {
      console.error(`    - ${r.method} ${r.path}  (openapi.yaml:${r.line})`);
    }
  }
  console.error(`\n  FAIL: /api/v1 contract mismatch (AP-033)`);
  process.exit(1);
}

main();
