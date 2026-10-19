package js

import (
	"encoding/json"
	"math"
	"os"
	"strings"
	"testing"
)

type fixture struct {
	Num     [][]any `json:"num"`
	Fixed   [][]any `json:"fixed"`
	Prec    [][]any `json:"prec"`
	Round   [][]any `json:"round"`
	Strings [][]any `json:"strings"`
	Math    [][]any `json:"math"`
}

func f64(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case string:
		switch x {
		case "$NaN":
			return math.NaN()
		case "$Infinity":
			return math.Inf(1)
		case "$-Infinity":
			return math.Inf(-1)
		case "$-0":
			return math.Copysign(0, -1)
		}
	}
	panic(v)
}

func same(a, b float64) bool {
	if math.IsNaN(a) {
		return math.IsNaN(b)
	}
	return a == b && math.Signbit(a) == math.Signbit(b)
}

func load(t *testing.T) fixture {
	data, err := os.ReadFile("testdata/fixture.json")
	if err != nil {
		t.Skip("no fixture")
	}
	var f fixture
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestNumbers(t *testing.T) {
	f := load(t)
	for _, c := range f.Num {
		if got := NumberToString(f64(c[0])); got != c[1].(string) {
			t.Errorf("String(%v) = %q, want %q", c[0], got, c[1])
		}
	}
	for _, c := range f.Round {
		if got := Round(f64(c[0])); !same(got, f64(c[1])) {
			t.Errorf("Math.round(%v) = %v, want %v", c[0], got, c[1])
		}
	}
	for _, c := range f.Fixed {
		if got := ToFixed(f64(c[0]), int(c[1].(float64))); got != c[2].(string) {
			t.Errorf("(%v).toFixed(%v) = %q, want %q", c[0], c[1], got, c[2])
		}
	}
	for _, c := range f.Prec {
		if got := ToPrecision(f64(c[0]), int(c[1].(float64))); got != c[2].(string) {
			t.Errorf("(%v).toPrecision(%v) = %q, want %q", c[0], c[1], got, c[2])
		}
	}
	for _, c := range f.Strings {
		s := c[0].(string)
		if got := ToNumber(s); !same(got, f64(c[1])) {
			t.Errorf("Number(%q) = %v, want %v", s, got, c[1])
		}
		if got := ParseFloat(s); !same(got, f64(c[2])) {
			t.Errorf("parseFloat(%q) = %v, want %v", s, got, c[2])
		}
		if got := ParseInt(s, 0); !same(got, f64(c[3])) {
			t.Errorf("parseInt(%q) = %v, want %v", s, got, c[3])
		}
		if got := ParseInt(s, 16); !same(got, f64(c[4])) {
			t.Errorf("parseInt(%q, 16) = %v, want %v", s, got, c[4])
		}
	}
}

func TestMath(t *testing.T) {
	f := load(t)
	names := []string{"sin", "cos", "atan2", "acos", "asin", "pow", "log10", "log2", "hypot", "sqrt"}
	bad := map[string]int{}
	for _, c := range f.Math {
		x := f64(c[0])
		clamp := math.Max(-1, math.Min(1, x))
		got := []float64{Sin(x), Cos(x), Atan2(x, 0.7), Acos(clamp), Asin(clamp), Pow(math.Abs(x), 0.37), Log10(math.Abs(x)), Log2(math.Abs(x)), Hypot(x, 3.1), math.Sqrt(math.Abs(x))}
		for i, g := range got {
			if !same(g, f64(c[i+1])) {
				bad[names[i]]++
			}
		}
	}
	if len(bad) > 0 {
		var parts []string
		for k, v := range bad {
			parts = append(parts, k+":"+NumberToString(float64(v)))
		}
		t.Errorf("math functions differ from V8 on %d of %d inputs: %s", len(bad), len(f.Math), strings.Join(parts, " "))
	}
}
