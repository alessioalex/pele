// Writes go/internal/util/testdata/whatwg.json: what Node's URL gives for the http(s) URLs that
// SanitizeURL hands to whatwgHref, or null where the constructor throws.
//   node go/scripts/whatwg-fixture.mjs go/internal/util/testdata/whatwg.json
//
// With --tables, writes go/internal/util/whatwg_tables.go instead: the IDNA tables of ada, the URL
// parser inside Node, read from the ada.cpp of the version this Node bundles.
//   curl -sLO https://github.com/ada-url/ada/releases/download/v$(node -p process.versions.ada)/ada.cpp
//   node go/scripts/whatwg-fixture.mjs --tables go/internal/util/whatwg_tables.go ada.cpp
import { readFileSync, writeFileSync } from 'node:fs';
import { deflateRawSync } from 'node:zlib';

if (process.argv[2] === '--tables') writeTables(process.argv[3], readFileSync(process.argv[4], 'utf8'));
else writeFixture(process.argv[2]);

function href(s) {
  let parsed;
  try {
    parsed = new URL(s);
  } catch {
    return null;
  }
  parsed.protocol = parsed.protocol.toLowerCase();
  parsed.hostname = parsed.hostname.toLowerCase();
  return parsed.toString();
}

function writeFixture(out) {
  const cases = [
    'http://example.com', 'https://example.com/', 'HTTP://EXAMPLE.COM/A?B#C', 'HtTpS://Example.Com:443/x',
    'http://example.com:80/', 'https://example.com:80/', 'http://example.com:443', 'http://example.com:8080/p',
    'http://example.com:0080', 'http://example.com:0', 'http://example.com:65535', 'http://example.com:65536',
    'http://example.com:99999999999999999999', 'http://example.com:', 'http://example.com:/x', 'http://example.com:8a',
    'http://example.com: 80', 'http://example.com:-1', 'http://example.com:+1', 'http://a:b:c/',
    'http://user@example.com', 'http://user:pass@example.com/', 'http://:pass@example.com', 'http://user:@example.com',
    'http://:@example.com', 'http://@example.com', 'http://a@b@c/', 'http://a:b@c:d@e/', 'http://u:p:q@h/',
    'http://ü:ö@h/', 'http://a b:c d@h/', 'http://a"<>`{}|^[]\\;=@h/', 'http://%40@h', 'http://a@/', 'http://a@',
    'http://@', 'http://a@:80', 'http://a/b@c', 'http://a?b@c', 'http://a#b@c', 'http://a\\b@c',
    'http://0x7f.1', 'http://3232235777', 'http://127.0.0.1', 'http://0177.0.0.1', 'http://0x7F.0.0.1/',
    'http://1.2.3', 'http://1.2', 'http://1', 'http://0', 'http://0x', 'http://0x.0x', 'http://0X1.0x2',
    'http://1.2.3.4.', 'http://1.2.3.4..', 'http://1.2.3.4.5', 'http://256.1.1.1', 'http://1.256.1.1',
    'http://1.1.1.256', 'http://1.1.65535', 'http://1.1.65536', 'http://1.16777215', 'http://1.16777216',
    'http://4294967295', 'http://4294967296', 'http://08', 'http://09.1', 'http://1..2', 'http://.1', 'http://1.',
    'http://a.1', 'http://1.a', 'http://0xg', 'http://0x1g.1', 'http://a.0x', 'http://a.0x1f', 'http://a.0xG',
    'http://00000000000000000000000001', 'http://0x00000000000000000000001', 'http://%31%32%37.0.0.1',
    'http://１２７.０.０.１', 'http://0x7f.1.', 'http://1.2.3.09', 'http://1.2.3.0x', 'http://999999999999999999999',
    'http://[::1]', 'http://[::1]:80/', 'http://[::1]:8080/x', 'http://[::]', 'http://[1:2:3:4:5:6:7:8]',
    'http://[0:0:1:0:0:0:1:0]/', 'http://[1:0:0:2:0:0:0:3]', 'http://[0:0:0:0:0:0:0:0]', 'http://[1::]',
    'http://[::ffff:192.168.0.1]', 'http://[::ffff:c0a8:1]', 'http://[::1.2.3.4]', 'http://[::1.2.3]',
    'http://[::1.2.3.4.5]', 'http://[::01.2.3.4]', 'http://[::1.2.3.256]', 'http://[1:2:3:4:5:6:1.2.3.4]',
    'http://[1:2:3:4:5:6:7:1.2.3.4]', 'http://[1:2:3:4:5:6:7:8:9]', 'http://[1::2::3]', 'http://[:1]',
    'http://[1:]', 'http://[12345::]', 'http://[ABCD:EF01::]', 'http://[g::1]', 'http://[::1', 'http://[::1]x',
    'http://x[::1]', 'http://[]', 'http://[%3A%3A1]', 'http://[::1/]', 'http://[::1?]', 'http://[::1]/[x]',
    'http://[0::0:0:0:0:0:1]', 'http://[1:0::0:1]', 'http://[::%31]', 'http://[ ::1]',
    'http://%41.com', 'http://%2e', 'http://%2E%2e', 'http://a%2Eb', 'http://%zz', 'http://a%', 'http://a%2',
    'http://a%00b', 'http://a%20b', 'http://%c3%bc.de', 'http://%C3%BC.de', 'http://%ff', 'http://%ef%bb%bfa.com',
    'http://%e2%80%8b', 'http://a%25b', 'http://%5b::1%5d', 'http://xn--%61',
    'http://münchen.de', 'http://ÉXAMPLE.com', 'http://例え.jp', 'http://пример.рф', 'http://ελληνικά.gr',
    'http://☃.net', 'http://💩.la', 'http://⌘.ws', 'http://faß.de', 'http://ΣΑΣ.gr', 'http://ς.gr', 'http://İ.com',
    'http://ẞ.de', 'http://Ⅻ.com', 'http://ǅ.com', 'http://ｅｘａｍｐｌｅ．ｃｏｍ', 'http://a。b｡c．d',
    'http://Ａ.com', 'http://℀', 'http://⒈x', 'http://⑴', 'http://＜', 'http://a­b', 'http://­',
    'http://﻿a', 'http://a​b', 'http://a‌b', 'http://a‍b', 'http://‌', 'http://क्‍',
    'http://क्‌ष', 'http://ب‌ب', 'http://a‌ب', 'http://é', 'http://́a', 'http://a.́',
    'http://abé́', 'http://ẹ́', 'http://ẹ́', 'http://각',
    'http://각', 'http://Å', 'http://Ω.com', 'http://豈', 'http://क़',
    'http://مثال.إختبار', 'http://עברית.il', 'http://aא', 'http://א1', 'http://1א', 'http://אְ', 'http://א١2',
    'http://א١', 'http://١', 'http://a١', 'http://אa', 'http://العربية.com',
    'http://xn--nxasmq6b.com', 'http://XN--MNCHEN-3YA.de', 'http://xn--mnchen-3ya.de', 'http://xn--abc',
    'http://xn--abc-.com', 'http://xn--', 'http://xn--a', 'http://xn--ls8h.la', 'http://xn--zz', 'http://xn--99999999',
    'http://xn--a-ecp.com', 'http://xn--tda', 'http://xn--1ug', 'http://xn--mnchen-3ya.xn--', 'http://ab--cd',
    'http://xn-a', 'http://axn--b', 'http://xn--55qx5d.cn', 'http://xn--mnchen-3ya.ü', 'http://xn--ü',
    'http://a b', 'http://a<b', 'http://a>b', 'http://a^b', 'http://a|b', 'http://a"b', 'http://a`b', 'http://a{b}',
    "http://a!$&'()*+,;=b", 'http://a_b', 'http://-a-', 'http://a~b', 'http://a..b', 'http://.', 'http://..',
    'http://a.', 'http://.a', 'http://A.B.C', 'http://a\u007fb', 'http://a\u0080b', 'http://a b', 'http://a　b',
    'http:example.com', 'http:/x', 'https:///x', 'http://', 'http:', 'https:', 'http:?x', 'http:#x', 'http:/',
    'http:////x', 'http:\\\\x', 'http:/\\/\\x', 'http://\\x', 'http:x/y', 'https:x:1', 'http:@x', 'http:a@',
    'http://a/b/c', 'http://a/b/../c', 'http://a/../../c', 'http://a/./b', 'http://a/b/.', 'http://a/b/..',
    'http://a/b/./', 'http://a/b/../', 'http://a/%2e/b', 'http://a/%2E%2e/b', 'http://a/.%2E/b', 'http://a/%2e./b',
    'http://a/b/%2e%2e', 'http://a/b/%2e', 'http://a/...', 'http://a/.../b', 'http://a/..b', 'http://a/b..',
    'http://a/%2e%2e%2f', 'http://a/b%2fc', 'http://a//b//', 'http://a/b\\c\\..\\d', 'http://a/b?c/../d',
    'http://a/b#c/../d', 'http://a/ b"<>`{}|^', 'http://a/ü/💩', 'http://a/%zz%', 'http://a/a b',
    'http://a/é?é#é', 'http://a/😀', 'http://a/\ud800', 'http://a/x\udc00y',
    'http://a?q=1&r= 2"\'<>`#f g"<>`', 'http://a?', 'http://a#', 'http://a?#', 'http://a/?a?b#c#d', 'http://a/?%#%',
    'http://a?é', 'http://a#é', 'http://a/p?\u0000', 'http://a/p#\u0001', 'http://a/p?{}|\\^[]',
    'http://a/p#{}|\\^[]', 'http://a/p?\u007f#\u007f',
    ' http://a/ ', '\u0000http://a/\u0001', '\u001fhttp://a/', 'http://a/ ', 'h\tt\ntp://a\r/b', 'http://a\t.b\n/c',
    'http:/\t/a', 'http://a:8\t0/', 'http://[::\n1]', 'http://a/p?q\tr#f\ns', '\thttp://a', 'http://a/\t',
    'http://a/ \u0000 ', 'http://a/\u000b', 'http://a\u000bb',
  ];
  let seed = 12345;
  const rnd = () => { seed = (seed * 1103515245 + 12345) % 2147483648; return seed / 2147483648; };
  const pick = (a) => a[Math.floor(rnd() * a.length)];
  const schemes = ['http:', 'https:', 'HTTP:', 'HtTpS:'];
  const slashes = ['//', '//', '//', '', '/', '///', '\\\\', '/\\', '////'];
  const users = ['', '', '', '', 'user@', 'u:p@', ':p@', 'u:@', '@', 'a@b@', 'u:p:q@', 'ü@', '%40@', 'a b@', ':@'];
  const hosts = ['example.com', 'EXAMPLE.COM', 'a', 'münchen.de', 'ÉXAMPLE.com', '例え.jp', 'пример.рф', 'ελληνικά.gr',
    'مثال.إختبار', 'עברית.il', '☃.net', '💩.la', 'xn--nxasmq6b.com', 'xn--abc', 'XN--MNCHEN-3YA.de', '0x7f.1',
    '3232235777', '127.0.0.1', '1.2.3', '0177.0.0.1', '0x', '1.2.3.4.', '256.1.1.1', '4294967296', '[::1]', '[::]',
    '[1:2:3:4:5:6:7:8]', '[::ffff:192.168.0.1]', '[1:0:0:2:0:0:0:3]', '[::1', '[g::1]', '%41.com', '%2e', '%zz',
    'a%2Eb', 'a b', 'a<b', 'a^b', 'a%00b', 'a­b', 'ｅｘａｍｐｌｅ．ｃｏｍ', 'a‌b', 'faß.de', 'ς.gr', 'a..b',
    '.', '', '1..2', '0x.0x', '09.1', 'a。b', 'é', '́a', 'a_b', 'xn--', 'ab--cd', '﻿a', '⒈x', 'İ',
    'a.b.c.d.e', 'x.0xff', '%C3%BC', 'א1', 'a١', 'ǅ'];
  const ports = ['', '', '', '', ':', ':80', ':443', ':8080', ':0', ':65535', ':65536', ':0080', ':8a', ': 1'];
  const paths = ['', '', '/', '/a/b', '/a/../b', '/./a', '/%2e/', '/%2E%2e/x', '/.%2e/', '/a/..', '/a/.', '\\a\\b',
    '/ a"<>`{}|^', '/ü', '/💩', '/%zz', '/a b', '//', '/.../', '/a/./../b/', '/a%2fb', '/..', '/.', '/a/%2e'];
  const queries = ['', '', '', '?', '?a=b', '?a b\'"<>`', '?ü', '?a?b', '?%', '?{}|^\\'];
  const frags = ['', '', '', '#', '#a b"<>`', '#ü', '#a#b', '#%', '#{}|^\\'];
  const noise = ['\t', '\n', '\r', ' ', '\u0000', '\u001f', ' ', '/', '\\', '.', ':', '@', '%', '#', '?', '[', ']'];
  for (let i = 0; i < 4200; i++) {
    const scheme = pick(schemes);
    let s = scheme + pick(slashes) + pick(users) + pick(hosts) + pick(ports) + pick(paths) + pick(queries) + pick(frags);
    if (rnd() < 0.15) {
      // Only tabs and newlines may go inside the scheme: anything else makes one SanitizeURL keeps.
      const c = pick(noise);
      const from = '\t\n\r'.includes(c) ? 1 : scheme.length;
      const at = from + Math.floor(rnd() * (s.length - from));
      s = s.slice(0, at) + c + s.slice(at);
    }
    if (rnd() < 0.05) s = pick([' ', '\u0001', '\t']) + s + pick([' ', '\u0002', '\n']);
    cases.push(s);
  }
  const seen = new Set();
  const rows = [];
  for (const s of cases) {
    if (seen.has(s)) continue;
    seen.add(s);
    rows.push([s, href(s)]);
  }
  writeFileSync(out, '[\n' + rows.map((r) => JSON.stringify(r)).join(',\n') + '\n]\n');
}

function writeTables(out, src) {
  // The section of the amalgamated source that a file was copied from.
  const file = (name) => {
    const start = src.indexOf(`/* begin file src/${name} */`);
    const end = src.indexOf(`/* end file src/${name} */`, start);
    if (start < 0 || end < 0) throw new Error(`ada.cpp has no ${name}`);
    return src.slice(start, end);
  };
  const body = (text, name) => {
    const m = new RegExp(`\\b${name}(\\[\\w*\\])+\\s*=\\s*\\{`).exec(text);
    if (!m) throw new Error(`ada.cpp has no ${name}`);
    let i = m.index + m[0].length;
    let depth = 1;
    const start = i;
    while (depth) {
      const c = text[i++];
      if (c === '{') depth++;
      else if (c === '}') depth--;
    }
    return text.slice(start, i - 1).replace(/\/\/[^\n]*/g, '');
  };
  const nums = (text, name) => body(text, name).match(/0x[0-9a-fA-F]+|\d+/g).map(Number);

  const tables = file('mapping_tables.cpp');
  const mappings = nums(tables, 'mappings');
  const ranges = nums(tables, 'table');
  // The mapping of one code point, as ada::idna::map does it: null where it is disallowed.
  const map = (cp) => {
    let lo = 0;
    let hi = ranges.length / 2 - 1;
    while (lo < hi) {
      const mid = (lo + hi + 1) >> 1;
      if (ranges[mid * 2] <= cp) lo = mid;
      else hi = mid - 1;
    }
    const d = ranges[lo * 2 + 1];
    const code = d & 0xff;
    if (code === 0) return [];
    if (code === 1) return [cp];
    if (code === 2) return null;
    const idx = (d >>> 8) & 0xffff;
    return mappings.slice(idx, idx + (d >>> 24));
  };

  const norm = file('normalization_tables.cpp');
  const decompIndex = nums(norm, 'decomposition_index');
  const decompBlock = nums(norm, 'decomposition_block');
  const decompData = nums(norm, 'decomposition_data');
  const cccIndex = nums(norm, 'canonical_combining_class_index');
  const cccBlock = nums(norm, 'canonical_combining_class_block');
  const compIndex = nums(norm, 'composition_index');
  const compBlock = nums(norm, 'composition_block');
  const compData = nums(norm, 'composition_data');
  const ccc = (cp) => cccBlock[cccIndex[cp >> 8] * 256 + (cp % 256)];
  const decomposition = (cp) => {
    const at = decompIndex[cp >> 8] * 257 + (cp % 256);
    const [d0, d1] = [decompBlock[at], decompBlock[at + 1]];
    const n = (d1 >> 2) - (d0 >> 2);
    return n > 0 && !(d0 & 1) ? decompData.slice(d0 >> 2, (d0 >> 2) + n) : [];
  };
  const pairs = [];
  for (let cp = 0; cp < 0x110000; cp++) {
    const at = compIndex[cp >> 8] * 257 + (cp % 256);
    for (let k = compBlock[at]; k < compBlock[at + 1]; k += 2) pairs.push([compData[k + 1], cp, compData[k]]);
  }
  pairs.sort((a, b) => a[0] - b[0]);

  const validity = file('validity.cpp');
  const combining = new Set(nums(validity, 'combining'));
  const join = new Map();
  for (const [name, v] of [['virama', 1], ['R', 2], ['L', 3], ['D', 4]]) {
    for (const cp of nums(validity, name)) {
      if (join.has(cp)) throw new Error(`U+${cp.toString(16)} has two joining classes`);
      join.set(cp, v);
    }
  }
  // Bidi classes, cut down to the ones the label check tells apart.
  const bidiClass = { L: 1, R: 2, AL: 2, AN: 3, EN: 4, NSM: 5, ES: 6, CS: 6, ET: 6, ON: 6, BN: 6 };
  const bidi = new Uint8Array(0x110000);
  for (const m of body(validity, 'dir_table').matchAll(/\{\s*(\w+),\s*(\w+),\s*direction::(\w+)\s*\}/g)) {
    for (let cp = Number(m[1]); cp <= Number(m[2]); cp++) bidi[cp] = bidiClass[m[3]] ?? 0;
  }

  // Code points whose properties are read: those that can be left after mapping and normalizing.
  const valid = new Uint8Array(0x110000);
  for (let cp = 0; cp < 0x110000; cp++) {
    const m = map(cp);
    if (m && m.length === 1 && m[0] === cp) valid[cp] = 1;
  }
  for (let cp = 0; cp < 0x110000; cp++) {
    const m = map(cp);
    if (m) for (const c of m) if (!valid[c]) throw new Error(`U+${cp.toString(16)} maps to a code point that is not valid`);
  }
  const used = valid.slice();
  for (const [c, a, b] of pairs) used[c] = used[a] = used[b] = 1;
  // whatwg.go decomposes with the composition pairs alone: check that they give what ada's own
  // decomposition table does wherever it can be read.
  const composite = new Map(pairs.map(([c, a, b]) => [c, [a, b]]));
  const decompose = (cp) => (composite.has(cp) ? [...decompose(composite.get(cp)[0]), composite.get(cp)[1]] : [cp]);
  for (let cp = 0; cp < 0x110000; cp++) {
    if (!used[cp] || (cp >= 0xac00 && cp <= 0xd7a3)) continue;
    const want = decomposition(cp);
    const got = decompose(cp);
    if (want.length ? want.join() !== got.join() : got.length !== 1) throw new Error(`U+${cp.toString(16)} decomposes differently`);
  }

  const bytes = [];
  const uvarint = (v) => {
    while (v >= 0x80) {
      bytes.push((v & 0x7f) | 0x80);
      v = Math.floor(v / 128);
    }
    bytes.push(v);
  };
  const zigzag = (v) => uvarint(v < 0 ? -2 * v - 1 : 2 * v);

  // Mapping runs: a length and a kind, then what the kind needs.
  const VALID = 0, DISALLOWED = 1, IGNORED = 2, DELTA = 3, PAIRS = 4, SEQ = 5;
  const kindOf = (cp) => {
    const m = map(cp);
    if (m === null) return [DISALLOWED];
    if (m.length === 0) return [IGNORED];
    if (m.length === 1 && m[0] === cp) return [VALID];
    if (m.length === 1) return [DELTA, m[0] - cp];
    return [SEQ, ...m];
  };
  const runs = [];
  for (let cp = 0; cp < 0x110000; ) {
    // Upper and lower case letters that alternate: the even ones map to the next one.
    let end = cp;
    while (kindOf(end).join() === `${DELTA},1` && kindOf(end + 1).join() === `${VALID}`) end += 2;
    if (end - cp >= 4) {
      runs.push([end - cp, PAIRS]);
      cp = end;
      continue;
    }
    const k = kindOf(cp);
    end = cp + 1;
    while (end < 0x110000 && kindOf(end).join() === k.join()) end++;
    runs.push([end - cp, ...k]);
    cp = end;
  }
  uvarint(runs.length);
  let at = 0;
  for (const [n, kind, ...arg] of runs) {
    uvarint(n * 8 + kind);
    if (kind === DELTA) zigzag(arg[0]);
    if (kind === SEQ) {
      uvarint(arg.length);
      let prev = at;
      for (const c of arg) {
        zigzag(c - prev);
        prev = c;
      }
    }
    at += n;
  }

  // Property runs over the code points that are read; the others take the value before them.
  const prop = (cp) => ccc(cp) | (bidi[cp] << 8) | ((combining.has(cp) ? 1 : 0) << 11) | ((join.get(cp) ?? 0) << 12);
  const props = [];
  for (let cp = 0; cp < 0x110000; cp++) {
    const v = used[cp] ? prop(cp) : props.length ? props[props.length - 1][1] : 0;
    if (props.length && props[props.length - 1][1] === v) props[props.length - 1][0]++;
    else props.push([1, v]);
  }
  uvarint(props.length);
  for (const [n, v] of props) {
    uvarint(n);
    uvarint(v);
  }

  uvarint(pairs.length);
  let prev = 0;
  for (const [c, a, b] of pairs) {
    uvarint(c - prev);
    zigzag(a - c);
    zigzag(b - a);
    prev = c;
  }

  const data = deflateRawSync(Buffer.from(bytes), { level: 9 }).toString('base64');
  const lines = data.match(/.{1,96}/g).map((l) => `\t"${l}"`).join(' +\n');
  writeFileSync(out, `// Code generated by scripts/whatwg-fixture.mjs --tables; DO NOT EDIT.

package util

// idnaData holds the IDNA tables of ada ${process.versions.ada}, the URL parser of Node ${process.versions.node}, deflated and
// in base 64: the UTS #46 mapping as runs of code points, then runs of the properties read after it
// (canonical combining class, bidi class, mark, virama and joining type), then the canonical
// compositions.
const idnaData = "" +
${lines}
`);
}
