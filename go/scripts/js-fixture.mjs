// Writes go/internal/js/testdata/fixture.json: what V8 gives for number formatting, parsing and Math.
//   node go/scripts/js-fixture.mjs go/internal/js/testdata/fixture.json
import { writeFileSync } from 'node:fs';
let seed = 12345;
const rnd = () => { seed = (seed * 1103515245 + 12345) % 2147483648; return seed / 2147483648; };
const nums = [0, -0, 1, -1, 0.5, -0.5, 1.5, 2.5, -2.5, 0.49999999999999994, 1e21, 1e-7, 123456789012345680000, 1.005, 0.000001, 0.0000001, 5e-324, 1.7976931348623157e308, 100, 1e15, 1e16, 123.456, -123.456, 4.35, 0.1+0.2, 1/3, 2/3, 1e300*10];
for (let i = 0; i < 3000; i++) {
  const e = Math.floor(rnd() * 40) - 20;
  nums.push((rnd() - 0.5) * Math.pow(10, e));
  nums.push(Math.round(rnd() * 100000) / 100 * (rnd() < 0.5 ? -1 : 1));
  nums.push(Math.round(rnd() * 1000) / 8);
}
const out = { num: [], fixed: [], prec: [], round: [], strings: [], math: [] };
for (const x of nums) {
  out.num.push([x, String(x)]);
  out.round.push([x, Math.round(x)]);
  for (const f of [0, 1, 2, 6]) out.fixed.push([x, f, x.toFixed(f)]);
  for (const p of [1, 3, 12]) out.prec.push([x, p, x.toPrecision(p)]);
  out.math.push([x, Math.sin(x), Math.cos(x), Math.atan2(x, 0.7), Math.acos(Math.max(-1,Math.min(1,x))), Math.asin(Math.max(-1,Math.min(1,x))), Math.pow(Math.abs(x), 0.37), Math.log10(Math.abs(x)), Math.log2(Math.abs(x)), Math.hypot(x, 3.1), Math.sqrt(Math.abs(x))]);
}
for (const s of ['', ' 12 ', '12px', '1e', '1e5', '.5', '5.', '-.5', '+5', '0x1F', '0x', '0b101', '0o17', '-0x10', 'Infinity', '-Infinity', '+Infinity', 'infinity', '1_000', '   3.5 ﻿', '3.5.5', 'abc', '-', '+', '00012', '1e400', '1e-400', '12abc', '0.0000001', ' 3', ' 0x10 ', '1.2e+3', '1.2E-3', '٣']) {
  out.strings.push([s, Number(s), parseFloat(s), parseInt(s), parseInt(s, 16)]);
}
writeFileSync(process.argv[2], JSON.stringify(out, (k, v) => typeof v === 'number' && !Number.isFinite(v) ? '$' + v : Object.is(v, -0) ? '$-0' : v));
