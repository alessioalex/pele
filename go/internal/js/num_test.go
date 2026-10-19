package js

import (
	"encoding/json"
	"math"
	"os"
	"strconv"
	"strings"
	"testing"
)

type fixture struct {
	Num     [][]any `json:"num"`
	Fixed   [][]any `json:"fixed"`
	Prec    [][]any `json:"prec"`
	Round   [][]any `json:"round"`
	Strings [][]any `json:"strings"`
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

// TestMath checks testdata/math.json: {"<fn> <fn>...": ["<inputs> <result per fn>", ...]},
// every number written as its float64 bits in hex.
func TestMath(t *testing.T) {
	data, err := os.ReadFile("testdata/math.json")
	if err != nil {
		t.Skip("no math fixture")
	}
	var tables map[string][]string
	if err := json.Unmarshal(data, &tables); err != nil {
		t.Fatal(err)
	}
	unary := map[string]func(float64) float64{
		"sin": Sin, "cos": Cos, "tan": Tan, "asin": Asin, "acos": Acos, "atan": Atan,
		"exp": Exp, "expm1": Expm1, "sinh": Sinh, "cosh": Cosh, "tanh": Tanh,
		"log": Log, "log2": Log2, "log10": Log10, "log1p": Log1p, "cbrt": Cbrt, "sqrt": math.Sqrt,
	}
	binary := map[string]func(float64, float64) float64{
		"atan2": Atan2, "pow": Pow, "hypot": func(x, y float64) float64 { return Hypot(x, y) },
	}
	for key, rows := range tables {
		names := strings.Fields(key)
		arity := 1
		if binary[names[0]] != nil {
			arity = 2
		}
		bad := make([]int, len(names))
		for _, row := range rows {
			fields := strings.Fields(row)
			if len(fields) != arity+len(names) {
				t.Fatalf("%s: bad row %q", key, row)
			}
			v := make([]float64, len(fields))
			for i, f := range fields {
				b, err := strconv.ParseUint(f, 16, 64)
				if err != nil {
					t.Fatalf("%s: bad row %q", key, row)
				}
				v[i] = math.Float64frombits(b)
			}
			for i, name := range names {
				var got float64
				if arity == 1 {
					got = unary[name](v[0])
				} else {
					got = binary[name](v[0], v[1])
				}
				if want := v[arity+i]; !same(got, want) {
					if bad[i] < 3 {
						t.Errorf("%s(%s) = %016x, want %016x", name, strings.Join(fields[:arity], ", "), math.Float64bits(got), math.Float64bits(want))
					}
					bad[i]++
				}
			}
		}
		for i, n := range bad {
			if n > 0 {
				t.Errorf("%s differs from V8 on %d of %d inputs", names[i], n, len(rows))
			}
		}
	}
}
