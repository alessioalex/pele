package kanban

// Temporary: compares common.TokenizeOutline/ParseOutline with the TypeScript. Delete after use.

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/alessioalex/pele/go/internal/core"
	"github.com/alessioalex/pele/go/internal/diagrams/common"
	"github.com/alessioalex/pele/go/internal/js"
)

type recDb struct{ calls []any }

func (d *recDb) GetType(start, end string) int {
	d.calls = append(d.calls, []any{"getType", start, end})
	return js.Len(start)*10 + js.Len(end)
}
func (d *recDb) AddNode(level int, id, descr string, typ int, shapeData *string) {
	var sd any
	if shapeData != nil {
		sd = *shapeData
	}
	d.calls = append(d.calls, []any{"addNode", level, id, descr, typ, sd})
}
func (d *recDb) DecorateNode(dec common.Decoration) {
	d.calls = append(d.calls, []any{"decorate", dec.Icon, dec.Class})
}

func TestOutlineScratch(t *testing.T) {
	in := os.Getenv("OUTLINE_IN")
	if in == "" {
		t.Skip()
	}
	data, _ := os.ReadFile(in)
	var srcs []string
	if err := json.Unmarshal(data, &srcs); err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	for _, src := range srcs {
		for _, kanban := range []bool{false, true} {
			tok := common.TokenizeOutline(src, kanban)
			starts := make([]int, len(tok.Starts))
			for i, s := range tok.Starts {
				starts[i] = js.UnitsBefore(src, s)
			}
			db := &recDb{calls: []any{}}
			var errRec any
			func() {
				defer func() {
					if r := recover(); r != nil {
						e := r.(*core.Error)
						errRec = map[string]any{"message": e.Message, "line": e.Line, "column": e.Column, "snippet": e.Snippet, "code": e.Code, "type": e.Type}
					}
				}()
				common.ParseOutline(src, db, kanban)
			}()
			b, _ := json.Marshal(map[string]any{"kanban": kanban, "tokens": map[string]any{"types": tok.Types, "texts": tok.Texts, "starts": starts}, "calls": db.calls, "error": errRec})
			out.Write(b)
			out.WriteByte('\n')
		}
	}
	os.WriteFile(os.Getenv("OUTLINE_OUT"), []byte(out.String()), 0o644)
}
