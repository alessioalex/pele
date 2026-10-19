# Porting a diagram type to Go

The guide each port followed. See [TODO.md](TODO.md) for what is left.

Pele renders Mermaid text to SVG. The library (`src/`) is being ported to Go in `go/` (module `github.com/alessioalex/pele/go`, Go 1.24 at `/usr/local/go/bin/go`).
**The goal is byte-identical output**: for the same text and options, the Go port must return exactly the
SVG string, width, height, links — or exactly the error message, code, line, column, snippet — that the
TypeScript library returns in Node with its default (metrics) text measurer. Read `AGENTS.md`
for the project conventions (they apply to Go code too: comment only non-obvious decisions, intentional
differences and preserved quirks; keep comments brief; no runtime dependencies — Go standard library only;
rendering synchronous and deterministic).


## Rules

- Only create/edit files in the package directories you were assigned (e.g. `go/internal/diagrams/pie/`).
- Fix a bug in a shared package minimally, then run `go test ./internal/engine/ ./internal/diagrams/info/ ./internal/js/` plus your own
  tests, and list the change in your report. Never refactor shared code.
- Never run git commands that change state (no commit, stash, checkout, reset, clean). Don't edit `go.mod`,
  don't add dependencies, don't run `gofmt -w`/`go fmt` on other agents' directories (`gofmt -l` your own).
- Don't modify anything under `src`, `tests/`, `dist/`, or `go/testdata/`. Scratch files go
  in `a scratch directory outside the repository`.
- Before finishing: `gofmt -l` and `go vet` clean for your packages; your golden test at 100%.

## Layout

- Each diagram type is a package `go/internal/diagrams/<dir>/` where `<dir>` is the TS directory name
  (`src/diagrams/<dir>/`). It exports `var Diagram = &core.Diagram{Type: ..., Section: ..., KeepComments: ...,
  Parse: ..., Render: ...}` mirroring `src/diagrams/<dir>/index.ts` (Type must equal the TS `type`, e.g.
  `gitGraph`, `quadrantChart`, `treeView`, `class`). Split files the way the TS does (lexer, parser, db, layout,
  render) so the two stay easy to compare; keep function names recognisable (Go style).
- Reference port: `go/internal/diagrams/info/` (passes all its records). Copy its test file pattern:
  ```go
  func TestGolden(t *testing.T) { golden.Run(t, "pie", engine.NewRegistry(Diagram)) }
  ```
  The name passed to `golden.Run` is the diagram's `type` (the recordings are split by it).

## Shared Go packages (TS equivalent → Go)

- `internal/js` — JS semantics. `js.Round` (Math.round; Go's math.Round differs on .5), `js.NumberToString` /
  `js.Num` (String(number), i.e. every `${x}` of a number in a template literal), `js.ToFixed`,
  `js.ToPrecision`, `js.ParseFloat`, `js.ToNumber` (Number(str)), `js.ParseInt(s, radix)` (0 = no radix),
  `js.Hypot`, `js.Sign`, `js.ToInt32` (`x|0`), `js.ToUint32`, `js.IsInteger`, `js.IsFinite`, `js.Cmp`
  (comparator result → int). `js.Sin/Cos/Atan2/Acos/Asin/Pow/Log10/Log2` (+ Tan, Atan, Exp, Log, …) are being
  made bit-identical to V8 by another agent — **always call these, never math.Sin etc.** (`**` is `js.Pow`).
  math.Floor/Ceil/Abs/Sqrt/Trunc/Mod and Go's builtin `min`/`max` on float64 match JS (NaN, -0) and are fine.
  Strings: `js.IsSpace` (JS `\s` / trim whitespace — NOT Go's unicode.IsSpace or regexp `\s`), `js.Trim`,
  `js.TrimStart`, `js.TrimEnd`, `js.IsLineEnd`, `js.IsWord`, `js.Len` (UTF-16 length), `js.Slice` (slice by
  UTF-16 indices), `js.UnitsBefore`, `js.ByteOffset`, `js.Units`/`js.FromUnits` ([]uint16), `js.CodeUnitAt`,
  `js.RuneAt`/`js.RuneBefore` (decode at a byte offset), `js.ToLower` (toLowerCase), `js.ToUpper`,
  `js.ASCIILower`, `js.HasPrefixFold` (ASCII case-insensitive prefix, as a regex `i` flag without `u`),
  `js.Repeat`.
- `internal/core` — `core.Options` (RenderOptions; nil pointer = undefined; helpers `FontSizeOr(16)`,
  `PaddingOr(8)`, `IDPrefixOr`, `Available()` = usable maxWidth, `NowOr()`, `Icon(name)`, `IconOr`;
  `Measurer` is always set by the engine — use `options.Measurer` wherever TS has
  `options.measurer ?? defaultMeasurer(...)`), `core.Rendered`, `core.LinkInfo`, `core.Diagram`,
  `core.ParseLimits` (MaxEdges float64, may be +Inf), `core.Error` + `core.SyntaxError(type, src, byteOffset,
  detail, lexical)` + `core.Locate` (byte offsets in, UTF-16 columns out) — throw with `panic(err)`; the engine
  recovers `*core.Error` and tags it with the type. `core.Object` (a JS object with JS key order: `Get`,
  `Value`, `Set`, `Keys`, `Section(key)`, `Clone`, `Assign`), config values are `nil|bool|float64|string|
  []core.Value|*core.Object`; `core.ToString` (String(v)), `core.ToNumber` (Number(v)), `core.Truthy`,
  `core.ParseJSON`, `core.ParseYaml`, `core.Version`.
- `internal/text` — `text.Measurer` interface, `text.Bold/Italic/Mono` (Style), `text.Layout` (layoutLabel),
  `text.Label`, `text.Span` (Icon "" = no icon), `text.LineHeight`, `text.HasCJK`, `text.DecodeEntities`,
  `text.IconMarkup(measurer, name)` (iconMarkup), `text.IconResolver` (`func(name) (markup, ok)`; ok=false is
  null/undefined), `text.Metrics`.
- `internal/svg` — `svg.Esc`, `svg.EscText`, `svg.Num` (num()), `svg.LabelSvg(label, cx, cy, attrs,
  options.Icons)`, `svg.SpanStyle`, `svg.SharedStyle`, `svg.TextStyle`, `svg.Document` (svgDocument; pass
  `svg.Accessible{AccTitle, AccDescr}`), `svg.WithTitle`, theme: `svg.RootStyle`, `svg.FontMono`,
  `svg.Radius`, `svg.Critical`, `svg.FontStyle`, `svg.ResolveStyle` (+ `svg.ResolvedStyle`, `FontSize` 0 =
  undefined, `HasStroke`), `svg.ClassNames`, `svg.SeriesColor`, `svg.Unsafe` (RE_UNSAFE).
  Coming soon from another agent (same package): ports of `svg/shapes.ts`, `svg/edges.ts`, `svg/cluster.ts`.
- `internal/util` — `util.SanitizeURL`, `util.SafeURL`, `util.LinkURL`, `util.ImageURL`, `util.Linked`,
  `util.RelAttr`, `util.DecodeURIComponent`, `util.IsUnicodeLetter`. (`src/util/chars.ts` → use `js.IsSpace`,
  `js.IsWord`, etc.; `isBlank(c)` is space or tab.)
- `internal/pre` — preprocess/detect (`pre.CleanupComments`, `pre.Merge`).
- `internal/diagrams/common` — `common/tokens.ts` → `common.TokenType` (`Literal` or `Match` scanner instead of
  a sticky regex; `Hidden`, `Opener`, `First`, `Longer`, `NewScanner`), `common.Tokenize`, `common.Tokens`,
  `common.Reader` (`Kind()`, `Take()`, `Accept`, `Expect`, `Fail`, field `I`), the shared terminals
  `common.ACC_DESCR`, `ACC_TITLE`, `TITLE`, `STRING`, `NEWLINE`, `WHITESPACE`, `YAML`, `DIRECTIVE`,
  `SINGLE_LINE_COMMENT`, `common.Keyword(word)`, `TitleValue`, `AccTitleValue`, `AccDescrValue`,
  `StringValue`, `CollapseBlanks`, `QuotedEnd`, `NewlineEnd`, `KeywordEnd`, `common.Re(goRegexp)`;
  `accDescr.ts` → `common.AccDescr(name)`; `fit-width.ts` → `common.FitWidth`, `TitleRoom`, `CapWidth`,
  `TurnToFit`, `RunsAcross`, `Tighten`; `fit.ts` → `common.PlainLabel`, `common.FitLabel`; `fit-text.ts` →
  `common.FitText` (+`Fitted`); `literal-label.ts` → `common.LiteralLabel`, `BreakLabel`, `LeftLabelSvg`;
  `generics.ts` → `common.ParseGenericTypes`; `icon.ts` → `common.IconSvg`; `caseless.ts` → `common.WordAt`;
  `scan.ts` → `common.ScanTokens`, `LineEnd`, `KeywordAt`, `SkipTrivia`, `Accessibility`.
  `outline.ts` is NOT ported yet (the mindmap/kanban agent ports it into `common/outline.go`).

## Semantics that bite

- **Strings**: Go strings are UTF-8 bytes; JS strings are UTF-16 code units. Use byte offsets internally and
  convert only where JS semantics depend on code units: `.length` used as a limit/count/width, `slice` by
  count, error columns (core.SyntaxError/Locate convert byte offsets for you), `Array.from(s)` = `[]rune(s)`,
  `charCodeAt` on non-ASCII. A lexer that only branches on ASCII can work on bytes, treating bytes ≥ 0x80 as
  "other" — but check every place it tests JS whitespace (`\s` includes U+00A0, U+3000, U+FEFF, U+2028, …),
  `\w`, letters, or code-unit counts. Astral characters are 2 code units in JS.
- **Regexes**: there is no lookahead/lookbehind/backreference in Go's regexp. Prefer hand-written matchers that
  reproduce the JS regex exactly, including what backtracking picks (greedy/lazy quantifiers, alternation
  order). Go regexp is OK only when semantics are provably identical: leftmost-first alternation matches what a
  backtracking engine picks, but Go's `\s` lacks `\v` and Unicode spaces, `.` matches `\r`/` `/` `
  (JS `.` does not), `(?i)` folds Unicode (ſ→s, K→k; JS without `u` folds ASCII only), `^`/`$` with `m` differ
  on `\r`/` `, and when matching `src[at:]` a leading `\b` cannot see the previous character. JS `split`
  with a regex, `replace` with `$1`/functions, global `exec` loops with `lastIndex` — port precisely.
- **Numbers**: everything numeric in JS is float64 — keep float64 for coordinates and values; ints only for
  indices/counts. Every number interpolated into SVG or text: `svg.Num` where TS calls `num()`, otherwise
  `js.NumberToString` (e.g. `${width}`, `${i}`; note `String(0.1+0.2)`). Integer division by zero panics in
  Go — JS gives Infinity/NaN. `Math.max(...[])` is -Infinity. Array index out of range is `undefined` in JS,
  a panic in Go: guard where TS relies on it.
- **Ordering**: `Array.prototype.sort` is stable: use `slices.SortStableFunc` / `sort.SliceStable`, with
  `js.Cmp(a - b)` for numeric comparators (NaN → equal). `.sort()` without a comparator compares as strings
  (UTF-16 order). `Map`/`Set`/object iteration is insertion order — Go maps are random: whenever TS iterates a
  Map/Set/object, keep an ordered slice of keys alongside the map. Object keys that look like array indices
  iterate first in numeric order (core.Object does this; plain `Record<string, …>` in TS models does too).
- **Dates** (gantt): JS Date in local time; recordings were made with `TZ=UTC`. Tests run with TZ unset; make the
  test set `time.Local = time.UTC` (or equivalent) and port Date arithmetic exactly (ms since epoch as float64,
  JS Date's own rules for overflowing fields, invalid dates as NaN).
- **Errors**: TS `throw new PeleError(...)` / `syntaxError(...)` → `panic(core.NewError(...))` /
  `panic(core.SyntaxError(...))`. Other JS exceptions (TypeError, RangeError) in TS code paths that tests reach
  were not recorded; if TS relies on catching its own exception, mirror it with recover in your package.
- **Stack depth**: records whose error is "Diagram is nested too deeply." are skipped by the harness (Go does not
  run out of stack like V8). Everything else must match.

## Testing

- Recordings of every `render()`/`parse()` the TS test suite made, plus the corpus with option variants, are in
  `go/testdata/recorded/<type>.jsonl` (one JSON per line: text, options, result or error). `golden.Run` replays
  them. `go test -run TestGolden ./internal/diagrams/<dir>/ -v` prints counts and the first failures (with
  the first differing byte of the SVG). Env: `PELE_GOLDEN_SHOW=n` (failures shown), `PELE_GOLDEN_ONLY=i` (run
  one record, full text), `PELE_GOLDEN_FILE=/abs/path.jsonl` (replay another file).
- Write extra cases / fuzz: put texts (or `{text, options}`) in a JSON array and run
  `TZ=UTC node go/scripts/record-inputs.mjs in.json out.jsonl` (uses `dist/pele.js`,
  already built from the current src), then `PELE_GOLDEN_FILE=$PWD/out.jsonl go test -run TestGolden ...`.
  Do a fuzz pass for your types (mutate corpus examples from `tests/corpus/<type>-docs.json` and the recorded
  texts: delete/duplicate/swap lines and characters, insert Unicode/CJK/emoji/`%%`/quotes/brackets/entities)
  and fix every mismatch. A few thousand fuzz cases per type is good.
- The TS tests (`tests/<type>/`, `tests/render/<type>*.test.ts`, `tests/compat/<type>/`) explain intended
  behaviour. Don't port them; the recordings already captured their render calls.
- Keep it fast: the TS code is linear-time on large inputs (stress tests exist in the records); avoid quadratic
  string concatenation (use strings.Builder) and accidental O(n²) scans.

## Report

When done, report: packages/files created, golden pass counts per type (passed/failed/skipped), fuzz cases run,
any known mismatch with its cause, and any change you made to shared packages.
