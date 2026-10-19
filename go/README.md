# Pele for Go

A Go port of [Pele](../README.md): it renders [Mermaid](https://mermaid.js.org) diagrams to SVG from Go,
with no dependencies outside the standard library. It draws exactly what the TypeScript library draws — the
same SVG, byte for byte, themed with the same CSS custom properties — for every diagram type built into
Mermaid 12.1.0.

## Install

```sh
go get github.com/alessioalex/pele/go
```

## Use

```go
import pele "github.com/alessioalex/pele/go"

res, err := pele.Render("flowchart LR\n  A[Mermaid text] --> B[SVG]", nil)
if err != nil {
	var perr *pele.Error
	if errors.As(err, &perr) {
		log.Printf("line %d, column %d: %s", perr.Line, perr.Column, perr.Message)
	}
	return err
}
os.WriteFile("diagram.svg", []byte(res.SVG), 0o644)
```

`Render` is synchronous, deterministic and safe for concurrent use. `Options` mirror the TypeScript
`RenderOptions`:

```go
res, err := pele.Render(src, &pele.Options{
	MaxWidth:   480,                         // draw narrower where the type can
	IDPrefix:   "d1",                        // several diagrams on one page
	FontFamily: "Inter",                     // the font labels are measured and drawn in
	Responsive: pele.Bool(false),            // a fixed size instead of max-width:100%
	Config:     map[string]any{"flowchart": map[string]any{"curve": "linear"}},
	Icons: func(name string) (string, bool) { // e.g. "fa:fa-user"
		return `<circle cx="12" cy="12" r="10"/>`, true
	},
})
```

Parse once and draw at several widths with `pele.Parse` and `(*Model).Render`. `pele.DetectType` and
`pele.Supports` tell what kind of diagram a text is.

Labels are measured with a table of Arial advance widths, as the TypeScript library does where it has no
canvas. Supply a `TextMeasurer` to measure in the font the SVG will be shown in.

Colors and fonts come from CSS variables, so a page can theme the SVG without drawing it again; see
[Theming](https://pele.run/theming).

### Command line

```sh
go run github.com/alessioalex/pele/go/cmd/pele diagram.mmd > diagram.svg
echo 'pie
  "Go" : 60
  "TypeScript" : 40' | go run github.com/alessioalex/pele/go/cmd/pele -max-width 400 -o pie.svg
```

## How it is tested

The Go port is checked against the TypeScript library, not against its own snapshots.
`scripts/record-golden.mjs` runs the TypeScript test suite with `render()` and `parse()` wrapped so that every
call is recorded with its options and its SVG or error, and draws every corpus example with a few option sets.
That is about 185,000 records (`testdata/recorded/`, not committed); `scripts/select-golden.mjs` keeps a
selection in `testdata/golden/`. Each diagram package's `TestGolden` replays them and requires identical output:

```sh
npm ci && npm run build           # in the repository root
node go/scripts/record-golden.mjs # optional: the full recording
cd go && go test ./...
```

To check other texts, write them to a JSON array and record what the TypeScript build draws:

```sh
TZ=UTC node go/scripts/record-inputs.mjs inputs.json /tmp/out.jsonl
PELE_GOLDEN_FILE=/tmp/out.jsonl go test -run TestGolden ./internal/diagrams/flowchart/
```

The port follows the TypeScript closely, module by module, so that a change to one can be carried to the other.
JavaScript semantics the output depends on — number formatting, `Math.round`, V8's `Math.sin` and friends,
UTF-16 string lengths, regular expression behaviour, stable sorting, insertion-ordered maps, `Date` — are
reproduced in `internal/js` and in each port.
