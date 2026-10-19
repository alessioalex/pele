package pele

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alessioalex/pele/go/internal/golden"
)

// Every recorded call of the TypeScript test suite and corpus, through the full registry.
func TestGolden(t *testing.T) {
	for _, typ := range append([]string{"_none"}, Types()...) {
		t.Run(typ, func(t *testing.T) { golden.Run(t, typ, registry) })
	}
}

func TestRender(t *testing.T) {
	res, err := Render("info\n  title Hello", nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Type != "info" || res.MermaidType != "info" || !strings.HasPrefix(res.SVG, `<svg xmlns="http://www.w3.org/2000/svg" class="pele pele-info"`) {
		t.Fatalf("unexpected result %+v", res)
	}
	fixed, err := Render("info", &Options{Responsive: Bool(false), IDPrefix: "x"})
	if err != nil || strings.Contains(fixed.SVG, "max-width") {
		t.Fatalf("responsive: false still scales: %v", err)
	}
}

func TestErrors(t *testing.T) {
	_, err := Render("not a diagram", nil)
	pe, ok := err.(*Error)
	if !ok || pe.Code != ErrUnsupported {
		t.Fatalf("want an unsupported-diagram error, got %v", err)
	}
	_, err = Render("info\nbad", nil)
	pe, ok = err.(*Error)
	if !ok || pe.Code != ErrSyntax || pe.Line != 2 || pe.Column != 1 || pe.Type != "info" {
		t.Fatalf("want a syntax error at 2:1, got %#v", err)
	}
	_, err = Render(strings.Repeat("x", 11), &Options{Limit: 10})
	if pe, ok := err.(*Error); !ok || pe.Code != ErrLimit {
		t.Fatalf("want a limit error, got %v", err)
	}
	if _, err = Render(strings.Repeat(" ", 60000)+"info", &Options{Limit: -1}); err != nil {
		t.Fatalf("a negative limit should mean none: %v", err)
	}
}

func TestParse(t *testing.T) {
	src := "---\ntitle: Parsed\n---\ninfo"
	m, err := Parse(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if m.Type() != "info" {
		t.Fatalf("type %q", m.Type())
	}
	a, err := m.Render(&Options{MaxWidth: 300})
	if err != nil {
		t.Fatal(err)
	}
	b, err := Render(src, &Options{MaxWidth: 300})
	if err != nil || a.SVG != b.SVG {
		t.Fatalf("a parsed model draws differently from its text: %v", err)
	}
	if DetectType(src) != "info" || !Supports(src) || Supports("nothing") {
		t.Fatal("detection")
	}
}

func TestConcurrent(t *testing.T) {
	srcs := []string{"info", "info\n  title A", "info\n  accTitle: B"}
	want := make([]string, len(srcs))
	for i, s := range srcs {
		r, err := Render(s, &Options{Now: time.Unix(0, 0)})
		if err != nil {
			t.Fatal(err)
		}
		want[i] = r.SVG
	}
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for k := 0; k < 50; k++ {
				i := k % len(srcs)
				r, err := Render(srcs[i], &Options{Now: time.Unix(0, 0)})
				if err != nil || r.SVG != want[i] {
					t.Errorf("concurrent render of %q differs", srcs[i])
					return
				}
			}
		}()
	}
	wg.Wait()
}
