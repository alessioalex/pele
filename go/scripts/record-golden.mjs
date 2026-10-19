// Records what the TypeScript renderer draws, for the Go port to match byte for byte.
//
// It wraps render() and parse() in src/core.ts for the length of one run of the test suite, so
// that every call a test makes is written down with its options and its SVG or error, then draws
// every corpus example with a few option sets. The records go to go/testdata/recorded/<type>.jsonl,
// which is not committed: select-golden.mjs picks the part of them that is.
//
//   node go/scripts/record-golden.mjs
import { spawnSync } from 'node:child_process';
import { mkdirSync, mkdtempSync, readFileSync, readdirSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

const root = new URL('../../', import.meta.url).pathname;
const corePath = join(root, 'src/core.ts');
const outDir = join(root, 'go/testdata/recorded');
const recDir = mkdtempSync(join(tmpdir(), 'pele-record-'));

const RECORDER = `
import { appendFileSync as __append } from 'node:fs';
import { threadId as __tid } from 'node:worker_threads';
import { metricsMeasurer as __metrics } from './text/measurer.js';

const __dir = process.env.PELE_RECORD;
const __texts = new WeakMap<object, { text: string; options: ParseOptions }>();
const __PARSE_KEYS = ['limit', 'maxEdges', 'config'];

function __json(value: unknown): string {
  return JSON.stringify(value, (_k, v) =>
    typeof v === 'number' && !Number.isFinite(v) ? '$' + String(v) : v instanceof Date ? v.getTime() : v
  );
}

// Options as JSON, or undefined when they hold something a record cannot carry, such as a measurer.
function __options(options: RenderOptions, icons: Record<string, string | null> | undefined): Record<string, unknown> | undefined {
  const out: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(options)) {
    if (value === undefined) continue;
    if (key === 'measurer') {
      if (value !== __metrics) return undefined;
      continue;
    }
    if (key === 'icons') {
      out.icons = icons;
      continue;
    }
    if (key === 'now') {
      out.now = value instanceof Date ? value.getTime() : value;
      continue;
    }
    if (typeof value === 'function') return undefined;
    out[key] = value;
  }
  return out;
}

function __write(text: string, options: Record<string, unknown>, run: () => RenderResult): void {
  const rec: Record<string, unknown> = { text, options };
  try {
    const r = run();
    rec.result = { type: r.type, mermaidType: r.mermaidType, svg: r.svg, width: r.width, height: r.height, links: r.links };
  } catch (e) {
    if (e instanceof PeleError) {
      rec.error = { message: e.message, code: e.code, type: e.type, mermaidType: e.mermaidType, line: e.line, column: e.column, snippet: e.snippet };
    } else {
      rec.crash = String(e);
    }
  }
  try {
    const line = __json(rec);
    if (line.length < 8_000_000) __append(__dir + '/' + process.pid + '-' + __tid + '.jsonl', line + '\\n');
  } catch {}
}

// Runs a render the way the caller asked for it, and records it when it can be replayed.
function __record(text: string, options: RenderOptions): void {
  if (!__dir || typeof text !== 'string') return;
  // Without a prototype, so that an icon named __proto__ is kept.
  const seen: Record<string, string | null> = Object.create(null);
  const given = options.icons;
  const opts: RenderOptions = { ...options };
  if (given) opts.icons = (name: string) => {
    let r: string | null | undefined;
    try { r = given(name); } catch { r = null; }
    seen[name] = typeof r === 'string' ? r : null;
    return r;
  };
  if (opts.now === undefined) opts.now = Date.now();
  const json = __options(opts, given ? seen : undefined);
  if (json === undefined) return;
  __write(text, json, () => __realRender(text, opts));
  if (given) (json as { icons?: unknown }).icons = seen;
}

export function parse(text: string, options: ParseOptions = {}): ParsedDiagramModel {
  if (__dir && typeof text === 'string') {
    const o: RenderOptions = {};
    for (const key of __PARSE_KEYS) if ((options as Record<string, unknown>)[key] !== undefined) (o as Record<string, unknown>)[key] = (options as Record<string, unknown>)[key];
    __record(text, o);
  }
  const model = __realParse(text, options);
  __texts.set(model, { text, options });
  return model;
}

export function render(source: string | DiagramModel, options: RenderOptions = {}): RenderResult {
  if (__dir) {
    if (typeof source === 'string') {
      __record(source, options);
    } else if (source && typeof source === 'object' && __texts.has(source)) {
      const { text, options: parsed } = __texts.get(source)!;
      const o: Record<string, unknown> = { ...options };
      for (const key of __PARSE_KEYS) delete o[key];
      for (const key of __PARSE_KEYS) if ((parsed as Record<string, unknown>)[key] !== undefined) o[key] = (parsed as Record<string, unknown>)[key];
      __record(text, o as RenderOptions);
    }
  }
  return __realRender(source, options);
}
`;

const original = readFileSync(corePath, 'utf8');
const patched =
  original
    .replace('export function parse(text: string', 'function __realParse(text: string')
    .replace('export function render(source: string', 'function __realRender(source: string') + RECORDER;
if (patched === original + RECORDER) throw new Error('src/core.ts no longer has the functions this script wraps.');

// Every corpus example, as a page draws it, at phone width, and at a fixed size.
const CORPUS_TEST = `
import { readFileSync, readdirSync } from 'node:fs';
import { it } from 'vitest';
import { render } from '../../src/index.js';

const NOW = Date.UTC(2026, 0, 15, 12);
const sets = [{ now: NOW }, { now: NOW, maxWidth: 320 }, { now: NOW, responsive: false, idPrefix: 'd' }];

it('records the corpus', { timeout: 600000 }, () => {
  for (const file of readdirSync('tests/corpus').sort()) {
    for (const src of JSON.parse(readFileSync('tests/corpus/' + file, 'utf8')) as string[]) {
      for (const o of sets) {
        try {
          render(src, o);
        } catch {}
      }
    }
  }
});
`;
const corpusTest = join(root, 'tests/scratch/record-corpus.test.ts');
mkdirSync(join(root, 'tests/scratch'), { recursive: true });

writeFileSync(corePath, patched);
writeFileSync(corpusTest, CORPUS_TEST);
try {
  const run = spawnSync('npx', ['vitest', 'run', '--exclude', 'tests/website/**', '--exclude', 'tests/browsers.test.ts'], {
    cwd: root,
    stdio: ['ignore', 'inherit', 'inherit'],
    env: { ...process.env, PELE_RECORD: recDir, TZ: 'UTC' },
  });
  if (run.status !== 0) console.warn('The test suite did not pass; the records it made are kept.');

} finally {
  writeFileSync(corePath, original);
  rmSync(corpusTest, { force: true });
}

// Merge, drop repeats, and split by diagram type.
const byType = new Map();
const keys = new Set();
for (const file of readdirSync(recDir).sort()) {
  for (const line of readFileSync(join(recDir, file), 'utf8').split('\n')) {
    if (line === '') continue;
    const rec = JSON.parse(line);
    if (rec.crash !== undefined) continue;
    const key = JSON.stringify([rec.text, rec.options]);
    if (keys.has(key)) continue;
    keys.add(key);
    const type = rec.result?.type ?? rec.error?.type ?? '_none';
    if (!byType.has(type)) byType.set(type, []);
    byType.get(type).push(line);
  }
}
rmSync(recDir, { recursive: true, force: true });
rmSync(outDir, { recursive: true, force: true });
mkdirSync(outDir, { recursive: true });
let total = 0;
for (const [type, lines] of [...byType].sort()) {
  writeFileSync(join(outDir, `${type}.jsonl`), lines.join('\n') + '\n');
  total += lines.length;
  console.log(`${type}: ${lines.length}`);
}
console.log(`${total} records in ${outDir}`);
