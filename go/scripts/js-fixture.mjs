// Writes go/internal/js/testdata/fixture.json (number formatting and parsing) and math.json
// next to it (Math results as exact float64 bit patterns), as V8 computes them.
//   node go/scripts/js-fixture.mjs go/internal/js/testdata/fixture.json
import { writeFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
let seed = 12345;
const rnd = () => { seed = (seed * 1103515245 + 12345) % 2147483648; return seed / 2147483648; };
const nums = [0, -0, 1, -1, 0.5, -0.5, 1.5, 2.5, -2.5, 0.49999999999999994, 1e21, 1e-7, 123456789012345680000, 1.005, 0.000001, 0.0000001, 5e-324, 1.7976931348623157e308, 100, 1e15, 1e16, 123.456, -123.456, 4.35, 0.1+0.2, 1/3, 2/3, 1e300*10];
for (let i = 0; i < 3000; i++) {
  const e = Math.floor(rnd() * 40) - 20;
  nums.push((rnd() - 0.5) * Math.pow(10, e));
  nums.push(Math.round(rnd() * 100000) / 100 * (rnd() < 0.5 ? -1 : 1));
  nums.push(Math.round(rnd() * 1000) / 8);
}
const out = { num: [], fixed: [], prec: [], round: [], strings: [] };
for (const x of nums) {
  out.num.push([x, String(x)]);
  out.round.push([x, Math.round(x)]);
  for (const f of [0, 1, 2, 6]) out.fixed.push([x, f, x.toFixed(f)]);
  for (const p of [1, 3, 12]) out.prec.push([x, p, x.toPrecision(p)]);
}
for (const s of ['', ' 12 ', '12px', '1e', '1e5', '.5', '5.', '-.5', '+5', '0x1F', '0x', '0b101', '0o17', '-0x10', 'Infinity', '-Infinity', '+Infinity', 'infinity', '1_000', '   3.5 ﻿', '3.5.5', 'abc', '-', '+', '00012', '1e400', '1e-400', '12abc', '0.0000001', ' 3', ' 0x10 ', '1.2e+3', '1.2E-3', '٣']) {
  out.strings.push([s, Number(s), parseFloat(s), parseInt(s), parseInt(s, 16)]);
}
writeFileSync(process.argv[2], JSON.stringify(out, (k, v) => typeof v === 'number' && !Number.isFinite(v) ? '$' + v : Object.is(v, -0) ? '$-0' : v));

// math.json: {"<fn> <fn>...": ["<inputs> <result per fn>", ...]}, every number as 16 hex digits.
let ms = 0x2545f491;
const u32 = () => { ms = (ms + 0x6d2b79f5) | 0; let t = Math.imul(ms ^ (ms >>> 15), 1 | ms); t ^= t + Math.imul(t ^ (t >>> 7), 61 | t); return (t ^ (t >>> 14)) >>> 0; };
const unif = () => u32() / 4294967296;
const ri = (a, b) => a + Math.floor(unif() * (b - a + 1));
const f64 = new Float64Array(1), w = new Uint32Array(f64.buffer), b64 = new BigUint64Array(f64.buffer);
const hex = x => { f64[0] = x; return b64[0].toString(16).padStart(16, '0'); };
const words = (hi, lo) => { w[1] = hi >>> 0; w[0] = lo >>> 0; return f64[0]; };
const randBits = () => words(u32(), u32());
const randExp = (e0, e1, sign = true) => words((sign && u32() & 1 ? 0x80000000 : 0) | ri(e0, e1) << 20 | u32() & 0xfffff, u32());
const subnormal = () => words(u32() & 0x80000000 | (u32() & 0xfffff) >>> ri(0, 20), u32());
const ulps = (x, k) => { f64[0] = x; b64[0] += BigInt(x < 0 ? -k : k); return f64[0]; }; // finite nonzero x only
const times = (n, f) => Array.from({ length: n }, f);

const specials = [0, -0, Infinity, -Infinity, NaN, 1, -1, 0.5, -0.5, 2, -2, 3, 10, 0.1, 1 / 3, 5e-324, -5e-324, 2.2250738585072014e-308, 1.7976931348623157e308,
  -1.7976931348623157e308, Math.PI, Math.PI / 2, Math.PI / 4, -Math.PI / 2, Math.E, Math.LN2, Math.SQRT2, Math.SQRT1_2, 22, 1e10, 1e22, 1e300, 1e-300,
  709.782712893384, -745.1332191019411, 710.4758600739439, 2 ** 53, 2 ** 64, 2 ** 66, 2 ** -27, 2 ** -28, 2 ** -54, 2 ** 1023, 2 ** -1022];
const near = specials.filter(x => Number.isFinite(x) && x !== 0).flatMap(x => [ulps(x, -1), ulps(x, 1)]);
// Inputs around high words the fdlibm code branches on, either sign.
const edges = his => his.flatMap(t => [-2, -1, 0, 1, 2].flatMap(d => [0, 0xffffffff, u32(), u32()].map(lo => words((t + d) | (u32() & 0x80000000), lo))));
const common = (n, his) => [...specials, ...near, ...edges(his), ...times(n, randBits), ...times(n / 5, subnormal)];

// Doubles x = q*2^E nearest to multiples of pi/2 (q from the continued fraction of frac(2^E*2/pi)).
const twoOverPi = 0xa2f9836e4e441529fc2757d1f534ddc0db6295993c439041fe5163abdebbc561b7246e3a424dd2e006492eea09d1921cfe1deb1cb129a73ee88235f52ebb4484e99c7026b45f7e413991d639835339f49c845f8bbdf9283b1ff897ffde05980fef2f118b5a0a6d1f6d367ecf27cb09b74f463f669e5fea2d7527bac7ebe5f17b3d0739f78a5292ea6bfb5fb11f8d5d0856033046fc7b6babf0cfbc209af4361da9e391615ee61b086599855f14a068408dffd8804d73273106061556ca73a8c960e27bc08c6bn;
const nearPio2 = [];
for (let E = -40; E <= 970; E += 20) {
  const P = 220n, D = 1n << P;
  let a = (twoOverPi >> (BigInt(1584 - E) - P)) & (D - 1n), b = D, k1 = 1n, k2 = 0n;
  while (a !== 0n) {
    const t = b / a;
    [a, b, k1, k2] = [b - t * a, a, t * k1 + k2, k1];
    if (k1 >= 1n << 53n) break;
    if (k1 >= 1n << 40n) nearPio2.push(Number(k1) * 2 ** E * (unif() < 0.5 ? -1 : 1));
  }
}

const math = {};
const table = (names, inputs, ...fns) => {
  math[names] = inputs.map(xs => [...xs, ...fns.map(f => f(...xs))].map(hex).join(' '));
};
const unary = xs => xs.map(x => [x]);

table('sin cos tan', unary([...common(100, [0x3fe921fb, 0x4002d97c, 0x413921fb, 0x3ff921fb, 0x400921fb, 0x4012d97c, 0x3e400000, 0x3fd33333, 0x3fe90000,
  0x3e300000, 0x3fe59428]), ...nearPio2, ...times(300, () => (unif() - 0.5) * 20), ...times(150, () => (unif() - 0.5) * 2000),
  ...times(150, () => randExp(0x3c0, 0x433)), ...times(150, () => randExp(0x41f, 0x7fe)),
  ...times(150, () => ulps(ri(1, 2 ** 19) * Math.PI / 2 * (unif() < 0.5 ? -1 : 1), ri(-2, 2)))]), Math.sin, Math.cos, Math.tan);
table('asin acos', unary([...common(60, [0x3ff00000, 0x3fe00000, 0x3c600000, 0x3e400000, 0x3fef3333]), ...times(300, () => unif() * 2 - 1), ...times(100, () => randExp(0x3c0, 0x3fe)),
  ...times(100, () => ulps([1, -1, 0.5, -0.5, 0.975, -0.975][ri(0, 5)], ri(-3000, 3000))), ...times(100, () => (0.95 + unif() * 0.05) * (unif() < 0.5 ? 1 : -1))]),
  Math.asin, Math.acos);
table('atan', unary([...common(60, [0x44100000, 0x3fdc0000, 0x3e400000, 0x3ff30000, 0x3fe60000, 0x40038000]), ...times(200, () => (unif() - 0.5) * 10), ...times(150, () => randExp(0x3a0, 0x460)),
  ...times(100, () => ulps([0.4375, 0.6875, 1.1875, 2.4375][ri(0, 3)] * (unif() < 0.5 ? 1 : -1), ri(-300, 300)))]), Math.atan);
const sp2 = [0, -0, Infinity, -Infinity, NaN, 1, -1, 0.7, -0.7, 5e-324, 1e300, -1e-300];
table('atan2', [...sp2.flatMap(y => sp2.map(x => [y, x])), ...times(300, () => [(unif() - 0.5) * 20, (unif() - 0.5) * 20]),
  ...times(250, () => [randBits(), randBits()]), ...times(200, () => { const x = (unif() - 0.5) * 100; return [x * 2 ** ri(-70, 70), x]; }),
  ...times(100, () => [(unif() - 0.5) * 20, 1]), ...times(100, () => [(unif() - 0.5) * 1000, 0.7])], Math.atan2);
const expIn = (range, his) => [...common(60, [0x40862e42, 0x3fd62e42, 0x3ff0a2b2, 0x3e300000, ...his]), ...times(300, () => (unif() - 0.5) * 2 * range), ...times(150, () => (unif() - 0.5) * 4),
  ...times(150, () => randExp(0x3a0, 0x409)), ...times(100, () => ri(-1100, 1100) * Math.LN2 + (unif() - 0.5) * 1e-9),
  ...times(100, () => ulps([709.782712893384, -745.1332191019411, -708.3964185322641, 710.4758600739439, 0.34657359027997264, 1.0397207708399179,
    38.816242111356935, 22][ri(0, 7)], ri(-100, 100)))];
table('exp expm1', unary(expIn(750, [0x4043687a, 0x3c900000])), Math.exp, Math.expm1);
table('sinh cosh tanh', unary(expIn(30, [0x3fd62e43, 0x3c800000, 0x40360000, 0x3ff00000])), Math.sinh, Math.cosh, Math.tanh);
table('log log2 log10 log1p', unary([...common(60, [0x00100000, 0x3ff00000, 0x40000000, 0x3fda827a, 0x3fd2bec4, 0x3e200000, 0x3c900000, 0x43400000,
  0x3ff6a09c, 0x3ff6a09e, 0x3fe6a09e, 0x3ff6147a, 0x3ff6b851]), ...times(300, () => randExp(0, 0x7fe, false)), ...times(100, () => unif() * 100),
  ...times(150, () => ulps(1, ri(-3000, 3000))), ...times(100, () => 1 + (unif() - 0.5) * 2 ** -ri(1, 40)), ...times(100, () => 10 ** ri(-323, 308)),
  ...times(100, () => unif() * 2 - 1), ...times(100, () => -1 + unif() * 2 ** -ri(1, 50)), ...times(100, () => randExp(0x3b0, 0x3ff)),
  ...times(100, () => ulps([Math.SQRT2, Math.SQRT1_2, -0.2928932188134524, 0.41421356237309503][ri(0, 3)], ri(-100, 100))),
  ...times(100, () => ulps(Math.SQRT2 * 2 ** ri(-1, 60) - 1, ri(-3000, 3000)))]),
  Math.log, Math.log2, Math.log10, Math.log1p);
table('cbrt sqrt', unary([...common(100, [0x00100000, 0x3ff00000]), ...times(200, () => (unif() - 0.5) * 2000), ...times(200, () => randExp(0, 0x7fe)),
  ...times(50, () => ri(-100000, 100000) ** 3)]), Math.cbrt, Math.sqrt);
const sp3 = [0, -0, Infinity, -Infinity, NaN, 1, -1, 0.5, -0.5, 2, -2, 3, -3, 1.5, 5e-324, 1e300, 1e-300, 2 ** 31 + 1, 2 ** 53 + 2, 2 ** 64 + 2 ** 12];
table('pow', [...sp3.flatMap(x => sp3.map(y => [x, y])), ...times(250, () => [unif() * 10, (unif() - 0.5) * 20]),
  ...times(200, () => [randExp(0, 0x7fe, false), (unif() - 0.5) * 4]), ...times(150, () => [(unif() - 0.5) * 200, ri(-60, 60)]),
  ...times(150, () => [(unif() - 0.5) * 200, ri(-60, 60) + 0.5]), ...times(100, () => [unif() * 100, [0.5, -0.5, 2, -2, 3, 1 / 3, 0.37, 1.5][ri(0, 7)]]),
  ...times(150, () => [ulps(1, ri(-2000, 2000)), (unif() < 0.5 ? -1 : 1) * 10 ** (unif() * 20)]),
  ...times(100, () => [randExp(0, 0x7fe, false), (unif() < 0.5 ? -1 : 1) * randExp(0x3c0, 0x7fe, false)]), ...times(150, () => [randBits(), randBits()]),
  ...times(50, () => [Math.abs(subnormal()), [0.5, -0.5, 2, 1.5][ri(0, 3)]]), ...times(20, () => [words(0, u32()), 0.5]),
  ...edges([0x3ff3988e, 0x3ffbb67a, 0x3ff00000, 0x3fefffff, 0x00100000]).map(x => [Math.abs(x), (unif() - 0.5) * 20]),
  ...edges([0x41e00000, 0x43f00000, 0x43400000]).map(y => [ulps(unif() < 0.5 ? 1 : -1, ri(-3, 3)), y]),
  ...times(150, () => { const x = 1 + unif() * 9; return [x, [1024, -1022, -1074, -1060][ri(0, 3)] / Math.log2(x) + (unif() - 0.5) * 0.01]; }),
  ...[[2, -1075], [0.5, 1075], [4, -537.5], [2, 1024], [0.5, -1024], [2, -1074.5], [32, -215]].flatMap(([x, y]) => [-1, 0, 1].map(k => [x, ulps(y, k)]))],
  Math.pow);
table('hypot', [...times(150, () => [(unif() - 0.5) * 2000, (unif() - 0.5) * 2000]), ...times(150, () => [randExp(0x300, 0x500), 3.1])], Math.hypot);
const tables = Object.entries(math).map(([k, rows]) => `${JSON.stringify(k)}: [\n${rows.map(r => JSON.stringify(r)).join(',\n')}\n]`);
writeFileSync(join(dirname(process.argv[2]), 'math.json'), `{\n${tables.join(',\n')}\n}\n`);
