// Draws inputs with the TypeScript build and writes them as golden records, for checking the Go
// port on texts the test suite does not have (fuzzing, a bug report).
//
//   npm run build
//   node go/scripts/record-inputs.mjs inputs.json out.jsonl
//   PELE_GOLDEN_FILE=$PWD/out.jsonl go test -run TestGolden ./internal/diagrams/pie/
//
// inputs.json is an array of diagram texts, or of { text, options } objects with JSON options.
import { readFileSync, writeFileSync } from 'node:fs';
import { PeleError, render } from '../../dist/pele.js';

const [input, output] = process.argv.slice(2);
if (!input || !output) throw new Error('usage: record-inputs.mjs inputs.json out.jsonl');

const NOW = Date.UTC(2026, 0, 15, 12);
const lines = [];
for (const item of JSON.parse(readFileSync(input, 'utf8'))) {
  const { text, options = {} } = typeof item === 'string' ? { text: item } : item;
  const opts = { now: NOW, ...options };
  const rec = { text, options: opts };
  try {
    const r = render(text, opts);
    rec.result = { type: r.type, mermaidType: r.mermaidType, svg: r.svg, width: r.width, height: r.height, links: r.links };
  } catch (e) {
    if (!(e instanceof PeleError)) continue;
    rec.error = { message: e.message, code: e.code, type: e.type, mermaidType: e.mermaidType, line: e.line, column: e.column, snippet: e.snippet };
  }
  lines.push(JSON.stringify(rec, (_k, v) => (typeof v === 'number' && !Number.isFinite(v) ? '$' + String(v) : v)));
}
writeFileSync(output, lines.join('\n') + '\n');
console.log(`${lines.length} records in ${output}`);
