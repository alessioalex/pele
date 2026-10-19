package util

import (
	"encoding/json"
	"os"
	"testing"
)

// The fixture is what Node's URL gives, from scripts/whatwg-fixture.mjs; null where it throws.
func TestWhatwgHref(t *testing.T) {
	raw, err := os.ReadFile("testdata/whatwg.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows [][2]*string
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	failed := 0
	for _, row := range rows {
		in, want := *row[0], row[1]
		got, ok := whatwgHref(in)
		if want == nil && !ok || want != nil && ok && got == *want {
			continue
		}
		failed++
		if failed <= 30 {
			if want == nil {
				t.Errorf("whatwgHref(%q) = %q, want failure", in, got)
			} else if !ok {
				t.Errorf("whatwgHref(%q) failed, want %q", in, *want)
			} else {
				t.Errorf("whatwgHref(%q) = %q, want %q", in, got, *want)
			}
		}
	}
	if failed > 0 {
		t.Errorf("%d of %d cases differ", failed, len(rows))
	}
}
