// Picks the part of go/testdata/recorded (written by record-golden.mjs) that is committed, as
// go/testdata/golden/<type>.jsonl.gz: every corpus example, then records in a fixed pseudo-random
// order until each type's budget is spent.
//
//   node go/scripts/select-golden.mjs
import { createHash } from 'node:crypto';
import { mkdirSync, readFileSync, readdirSync, rmSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { gzipSync } from 'node:zlib';

const root = new URL('../../', import.meta.url).pathname;
const from = join(root, 'go/testdata/recorded');
const to = join(root, 'go/testdata/golden');
const BUDGET = 1_500_000;
const LARGEST = 150_000;

const corpus = new Set();
for (const file of readdirSync(join(root, 'tests/corpus'))) {
  for (const text of JSON.parse(readFileSync(join(root, 'tests/corpus', file), 'utf8'))) corpus.add(text);
}

// A lone surrogate does not survive a JSON round trip into Go strings.
const RE_LONE = /[\ud800-\udbff](?![\udc00-\udfff])|(?<![\ud800-\udbff])[\udc00-\udfff]/;
const hash = (line) => createHash('sha1').update(line).digest('hex');

rmSync(to, { recursive: true, force: true });
mkdirSync(to, { recursive: true });
let total = 0;
for (const file of readdirSync(from).filter((f) => f.endsWith('.jsonl')).sort()) {
  const lines = readFileSync(join(from, file), 'utf8').split('\n').filter(Boolean);
  const picked = [];
  const rest = [];
  for (const line of lines) {
    const rec = JSON.parse(line);
    if (RE_LONE.test(rec.text) || (rec.result && RE_LONE.test(rec.result.svg))) continue;
    if (corpus.has(rec.text) && line.length < LARGEST * 4) picked.push(line);
    else if (line.length < LARGEST) rest.push(line);
  }
  rest.sort((a, b) => (hash(a) < hash(b) ? -1 : 1));
  let size = 0;
  for (const line of rest) {
    if (size + line.length > BUDGET) continue;
    picked.push(line);
    size += line.length;
  }
  const gz = gzipSync(picked.join('\n') + '\n', { level: 9 });
  writeFileSync(join(to, file + '.gz'), gz);
  total += gz.length;
  console.log(`${file}: ${picked.length} of ${lines.length} records, ${(gz.length / 1024).toFixed(0)} KB`);
}
console.log(`${(total / 1024 / 1024).toFixed(1)} MB in ${to}`);
