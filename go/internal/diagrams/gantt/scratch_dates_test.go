package gantt

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"testing"
	"time"
)

const scratch = "/tmp/claude-0/-home-user-pele/5fb8cf2f-759f-57b9-a094-9836ce1c612e/scratchpad/gantt/"

func numOf(v any) float64 {
	if s, ok := v.(string); ok && s == "NaN" {
		return math.NaN()
	}
	return v.(float64)
}

func same(a, b float64) bool { return a == b || (math.IsNaN(a) && math.IsNaN(b)) }

func TestScratchDates(t *testing.T) {
	time.Local = time.UTC
	b, err := os.ReadFile(scratch + "dates.json")
	if err != nil {
		t.Skip()
	}
	var cases [][2]any
	json.Unmarshal(b, &cases)
	fails := 0
	for _, c := range cases {
		s := c[0].(string)
		want := numOf(c[1])
		got := parseDateString(s).t
		if !same(got, want) {
			fails++
			if fails < 30 {
				t.Errorf("%q: got %v want %v", s, got, want)
			}
		}
	}
	t.Logf("%d/%d failed", fails, len(cases))

	b, _ = os.ReadFile(scratch + "ops.json")
	var ops []struct {
		Vals                []float64
		Base                float64
		D, U, A, B, C, E, F any
		Fields              []any
	}
	json.Unmarshal(b, &ops)
	fails = 0
	for _, o := range ops {
		v := o.Vals
		check := func(name string, got float64, want any) {
			if !same(got, numOf(want)) {
				fails++
				if fails < 30 {
					t.Errorf("%s vals=%v base=%v: got %v want %v", name, v, o.Base, got, numOf(want))
				}
			}
		}
		check("ctor", newLocalDate(v[0], v[1], v[2], v[3], v[4], v[5], v[6]).t, o.D)
		check("utc", dateUTC(v[0], v[1], v[2], v[3], v[4], v[5], v[6]), o.U)
		d := newDate(o.Base)
		d.setDate(v[2])
		check("setDate", d.t, o.A)
		d = newDate(o.Base)
		d.setMonth(v[1])
		check("setMonth", d.t, o.B)
		d = newDate(o.Base)
		d.setFullYear(v[0])
		check("setFullYear", d.t, o.C)
		d = newDate(o.Base)
		d.setHours(v[3], v[4], v[5], v[6])
		check("setHours", d.t, o.E)
		d = newDate(o.Base)
		d.setMonthDate(v[1], v[2])
		check("setMonthDate", d.t, o.F)
		g := newDate(o.Base)
		got := []float64{g.getFullYear(), g.getMonth(), g.getDate(), g.getDay(), g.getHours(), g.getMinutes(), g.getSeconds(), g.getMilliseconds(), g.getUTCSeconds()}
		for i, f := range o.Fields {
			check(fmt.Sprint("field", i), got[i], f)
		}
	}
	t.Logf("ops: %d failed", fails)
}
