# Go port: status and next steps

Keep this file current: update a row when a type's state changes. "Passes" means every recorded case
of that type gives identical output (`go test -run TestGolden ./internal/diagrams/<dir>/`).
[PORTING.md](PORTING.md) is the guide for porting a type; [README.md](README.md) explains the tests.

## Getting set up

```sh
npm ci && npm run build                 # repository root: the TypeScript library the port is checked against
node go/scripts/record-golden.mjs       # ~4 min: full recordings in go/testdata/recorded/ (not committed)
cd go && go test ./...
```

Without the full recordings, tests use the committed selection in `go/testdata/golden/`.

## Shared packages

| Package | State |
| --- | --- |
| `internal/js` | Done. Number formatting/parsing match V8; `Sin`, `Cos`, `Pow`, … are bit-exact with V8 (fixture `testdata/math.json`). |
| `internal/core`, `text`, `pre`, `engine`, `golden` | Done. All 37,781 pre-detection records pass (`go test ./internal/engine/`). |
| `internal/util` | Done. `whatwg.go` serializes http(s) URLs as `new URL()` does (fixture `testdata/whatwg.json`). |
| `internal/svg` | builder, theme, root: done. `shapes.go`, `edges.go`, `cluster.go`: ported; no fixture test of their own yet (they are checked through the diagrams that pass). TODO: confirm `cluster.go` is complete against `src/svg/cluster.ts`. |
| `internal/layout` | **In progress.** `layered.go` written (port of `src/layout/layered.ts`), untested. TODO: check it against the TS, then port `compound.ts` and `lanes.ts`. Blocks flowchart, swimlane, state, class, er, requirement, usecase, agentflow. |
| `internal/diagrams/common` | Done, including `outline.go` (mindmap, kanban). |
| `pele` (public API), `cmd/pele` | Done. Register each type in `registry.go` once it passes. |

## Diagram types

| Type (`dir`) | State |
| --- | --- |
| info | Passes |
| pie | Passes |
| packet | Passes |
| radar | Passes |
| gitGraph (`git`) | Passes |
| quadrantChart (`quadrant`) | Passes |
| xychart | Passes |
| venn | Passes |
| timeline | Passes |
| ishikawa | Passes |
| cynefin | Passes |
| eventmodeling | Passes |
| railroad | Passes |
| sankey | Passes (2 records skipped: "nested too deeply" in V8) |
| kanban | Passes |
| mindmap | Passes |
| treeView (`treeview`) | Passes |
| treemap | Passes |
| journey | Passes |
| gantt | In progress: JS `Date` emulation (`jsdate.go`), date parsing (`dateparse.go`, checked by `scratch_dates_test.go`) and the lexer (`lexer.go`, `tokens.go`; matched the TS lexer token for token on all 4,815 recorded inputs) are done. Parser, db, axis and render to do. |
| wardley | Passes |
| architecture | Passes |
| sequence | In progress: lexer, parser, db; renderer missing. Files carry `//go:build wip`. |
| block | In progress: lexer, parser, db, layout, render written; no golden test yet. |
| c4 | In progress: lexer, parser, db, layout, route; renderer missing. |
| flowchart | In progress: `shapes.go` (also used by agentflow), lexer, tokens; parser, db, graph, render to do (render needs `internal/layout`). |
| swimlane | Not started (thin layer over flowchart + `layout/lanes`). |
| agentflow | In progress: lexer, tokens. |
| state | Not started (needs layout). |
| class | Not started (needs layout). |
| er | Not started (needs layout). |
| requirement | Not started (needs layout). |
| usecase | Not started (needs layout). |

## Next steps, in order

1. Every type marked "Passes" is registered in `registry.go`; register each new one as it passes and run
   `go test .` (the public package replays every registered type).
2. Finish `internal/layout` (layered → compound → lanes), with a fixture test made by a TS driver
   (see PORTING.md, "Testing").
3. Finish the in-progress types, smallest first: gantt, sequence,
   block, c4; then the layout users: flowchart, swimlane, state, er, requirement, class, usecase, agentflow.
4. When every type passes: run the full suite against the full recordings, fuzz each type
   (`scripts/record-inputs.mjs`), remove `//go:build wip` tags and scratch tests, and update README.md.

## Known issues

- Records with a lone UTF-16 surrogate in them are skipped: a Go string cannot hold one, so the input
  cannot be replayed as the TypeScript saw it.
- Records whose TypeScript result is "Diagram is nested too deeply." are skipped: Go does not run out of
  stack where V8 does, so the Go port draws those diagrams.
- `scripts/record-golden.mjs` must be rerun when `src/` changes, and `scripts/select-golden.mjs` after it,
  so the committed selection follows the TypeScript.
