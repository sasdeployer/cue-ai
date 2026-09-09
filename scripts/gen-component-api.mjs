#!/usr/bin/env node
// Emit the AI-facing API surface of the deck engine + component library.
//
// The model authors ONLY App.tsx and is explicitly told never to rewrite a
// component, so it needs each component's doc comment, exported prop types and
// call signature — and nothing else. Dumping full component source instead cost
// ~21k input tokens (83KB) on EVERY generation, re-sent on every agent round and
// every compile retry, for implementation detail the model must not touch.
//
// Output replaces the old inline components.full.txt dump. Run from repo root
// (dev.sh does this); writes server/reference/components.api.txt.

import { readFileSync, writeFileSync, readdirSync } from 'node:fs'
import { join, basename } from 'node:path'

const root = process.cwd()
const files = [
  ...['Deck', 'Slide', 'Build', 'Reveal'].map((n) => join('src/deck', `${n}.tsx`)),
  ...readdirSync(join(root, 'src/components'))
    .filter((f) => f.endsWith('.tsx'))
    .sort()
    .map((f) => join('src/components', f)),
]

// matchDelims returns the index just past the delimiter that closes the one at
// `open`, honouring nesting, strings and comments. Prop types here are real TS
// ({ a: T; b?: U }, unions, tuples, nested objects) and span multiple lines, so
// a regex or a split on ';' truncates them — which is exactly how the previous
// components.api.txt ended up with `export type AccordionItem = { title: string;`.
function matchDelims(src, open, pair) {
  const [L, R] = pair
  let depth = 0
  for (let i = open; i < src.length; i++) {
    const c = src[i]
    if (c === '"' || c === "'" || c === '`') {
      const q = c
      i++
      while (i < src.length && src[i] !== q) i += src[i] === '\\' ? 2 : 1
      continue
    }
    if (c === '/' && src[i + 1] === '/') {
      while (i < src.length && src[i] !== '\n') i++
      continue
    }
    if (c === '/' && src[i + 1] === '*') {
      i = src.indexOf('*/', i + 2)
      if (i < 0) return src.length
      i++
      continue
    }
    if (c === L) depth++
    else if (c === R) {
      depth--
      if (depth === 0) return i + 1
    }
  }
  return src.length
}

// firstDocComment returns the file's leading /* … */ block, which is where every
// component documents itself and — crucially — shows a worked usage example.
// Position varies: above the imports (Comparison), between the imports and the
// first export (Accordion), or above a private helper that precedes the export
// (BrowserFrame, CodeWindow, VisualDashboard). Anchoring on the export or the
// function missed 18 and 5 of 31 files respectively, so take the first block
// comment wherever it sits. Line comments are skipped: they annotate code, while
// the /* */ block is the component's contract.
function firstDocComment(src) {
  const start = src.indexOf('/*')
  if (start < 0) return ''
  const end = src.indexOf('*/', start + 2)
  if (end < 0) return ''
  return src
    .slice(start + 2, end)
    .split('\n')
    .map((l) => l.replace(/^\s*\*?\s?/, '').trimEnd())
    .filter((l, i, a) => l !== '' || (i > 0 && i < a.length - 1))
    .join('\n')
    .trim()
}

// exportedTypes pulls complete `export type X = …` declarations, brace-balanced.
function exportedTypes(src) {
  const out = []
  const re = /export\s+type\s+(\w+)\s*=\s*/g
  let m
  while ((m = re.exec(src))) {
    const valueStart = m.index + m[0].length
    const brace = src.indexOf('{', valueStart)
    const semi = src.indexOf(';', valueStart)
    let end
    if (brace >= 0 && (semi < 0 || brace < semi)) {
      // Object/union type: walk the braces, then take the trailing ';'.
      end = matchDelims(src, brace, ['{', '}'])
      const tail = src.indexOf(';', end)
      if (tail >= 0 && tail <= end + 2) end = tail + 1
    } else {
      end = semi >= 0 ? semi + 1 : src.length
    }
    out.push(src.slice(m.index, end).replace(/\s*\n\s*/g, ' ').trim())
  }
  return out
}

// signatures pulls every exported function's name + full parameter list. Charts
// exposes three NAMED exports (BarChart/LineChart/DonutChart) rather than a
// default, and the system prompt calls them out specifically, so named exports
// must be captured, not just `export default`.
function signatures(src) {
  const out = []
  const re = /export\s+(?:default\s+)?function\s+(\w+)\s*\(/g
  let m
  while ((m = re.exec(src))) {
    const paren = src.indexOf('(', m.index + m[0].length - 1)
    const end = matchDelims(src, paren, ['(', ')'])
    const params = src
      .slice(paren, end)
      .replace(/\s*\n\s*/g, ' ')
      .replace(/\s{2,}/g, ' ')
      .trim()
    const isDefault = /export\s+default/.test(m[0])
    out.push({ name: m[1], params, isDefault, at: m.index })
  }
  return out
}

const parts = []
for (const rel of files) {
  const src = readFileSync(join(root, rel), 'utf8')
  const name = basename(rel, '.tsx')
  const sigs = signatures(src)
  const types = exportedTypes(src)

  const lines = [`### ${name}`]
  const doc = firstDocComment(src)
  if (doc) lines.push(doc)
  for (const t of types) lines.push(t)
  for (const s of sigs) {
    lines.push(`${s.isDefault ? 'default export' : 'named export'}: ${s.name}${s.params}`)
  }
  parts.push(lines.join('\n'))
}

const out = parts.join('\n\n') + '\n'
const dest = join(root, 'server/reference/components.api.txt')
writeFileSync(dest, out)
console.log(
  `gen-component-api: ${files.length} files → ${dest} (${out.length} bytes)`
)
