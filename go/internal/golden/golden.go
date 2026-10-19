// Package golden replays what the TypeScript library drew, recorded by
// scripts/record-golden.mjs, and checks that the Go port draws the same.
package golden

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/alessioalex/pele/go/internal/core"
	"github.com/alessioalex/pele/go/internal/engine"
)

// Record is one recorded call.
type Record struct {
	// Lone is set when the record holds a lone UTF-16 surrogate, which a Go string cannot hold.
	Lone    bool                       `json:"-"`
	Text    string                     `json:"text"`
	Options map[string]json.RawMessage `json:"options"`
	Result  *struct {
		Type        string `json:"type"`
		MermaidType string `json:"mermaidType"`
		SVG         string `json:"svg"`
		Width       Number `json:"width"`
		Height      Number `json:"height"`
		Links       []struct {
			ID       string `json:"id"`
			Href     string `json:"href"`
			Internal bool   `json:"internal"`
		} `json:"links"`
	} `json:"result"`
	Error *struct {
		Message     string  `json:"message"`
		Code        string  `json:"code"`
		Type        *string `json:"type"`
		MermaidType *string `json:"mermaidType"`
		Line        int     `json:"line"`
		Column      int     `json:"column"`
		Snippet     string  `json:"snippet"`
	} `json:"error"`
}

// Number is a recorded number, which the recorder writes as a string when it is not finite.
type Number float64

func (n *Number) UnmarshalJSON(b []byte) error {
	f, ok := number(b)
	if !ok {
		return fmt.Errorf("not a number: %s", b)
	}
	*n = Number(f)
	return nil
}

func dir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata")
}

// Load reads the records of a diagram type: those in PELE_GOLDEN_FILE when it is set, else the
// full recording when it is there (it is not committed), else the committed selection.
// PELE_GOLDEN=committed skips the full recording.
func Load(typ string) ([]Record, string, error) {
	var candidates []string
	if file := os.Getenv("PELE_GOLDEN_FILE"); file != "" {
		candidates = append(candidates, file)
	}
	if os.Getenv("PELE_GOLDEN") != "committed" {
		candidates = append(candidates, filepath.Join(dir(), "recorded", typ+".jsonl"))
	}
	candidates = append(candidates, filepath.Join(dir(), "golden", typ+".jsonl.gz"))
	for _, path := range candidates {
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		defer f.Close()
		var r io.Reader = f
		if strings.HasSuffix(path, ".gz") {
			gz, err := gzip.NewReader(f)
			if err != nil {
				return nil, path, err
			}
			r = gz
		}
		var out []Record
		sc := bufio.NewScanner(r)
		sc.Buffer(make([]byte, 1<<20), 64<<20)
		for sc.Scan() {
			line := sc.Bytes()
			if len(bytes.TrimSpace(line)) == 0 {
				continue
			}
			var rec Record
			if err := json.Unmarshal(line, &rec); err != nil {
				return nil, path, err
			}
			// JSON.stringify escapes only lone surrogates; a pair is written as its character.
			rec.Lone = reLone.Match(line)
			out = append(out, rec)
		}
		return out, path, sc.Err()
	}
	return nil, "", fmt.Errorf("no records for %s", typ)
}

var reLone = regexp.MustCompile(`\\ud[89a-f][0-9a-f]{2}`)

func number(raw json.RawMessage) (float64, bool) {
	var f float64
	if json.Unmarshal(raw, &f) == nil {
		return f, true
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		switch s {
		case "$Infinity":
			return math.Inf(1), true
		case "$-Infinity":
			return math.Inf(-1), true
		case "$NaN":
			return math.NaN(), true
		}
	}
	return 0, false
}

// restore turns the recorder's markers for non-finite numbers back into numbers.
func restore(v core.Value) core.Value {
	switch x := v.(type) {
	case string:
		switch x {
		case "$Infinity":
			return math.Inf(1)
		case "$-Infinity":
			return math.Inf(-1)
		case "$NaN":
			return math.NaN()
		}
	case []core.Value:
		for i := range x {
			x[i] = restore(x[i])
		}
	case *core.Object:
		for _, k := range x.Keys() {
			x.Set(k, restore(x.Value(k)))
		}
	}
	return v
}

// Options turns recorded options into Go options. ok is false for options the port cannot replay.
func Options(raw map[string]json.RawMessage) (*core.Options, bool) {
	o := &core.Options{}
	for key, value := range raw {
		var ok bool
		num := func(dst **float64) {
			var f float64
			if f, ok = number(value); ok {
				*dst = &f
			}
		}
		str := func(dst **string) {
			var s string
			if ok = json.Unmarshal(value, &s) == nil; ok {
				*dst = &s
			}
		}
		boolean := func(dst **bool) {
			var b bool
			if ok = json.Unmarshal(value, &b) == nil; ok {
				*dst = &b
			}
		}
		strs := func(dst *[]string) {
			var s []string
			if ok = json.Unmarshal(value, &s) == nil && s != nil; ok {
				*dst = s
			}
		}
		switch key {
		case "fontFamily":
			str(&o.FontFamily)
		case "fontFamilyMono":
			str(&o.FontFamilyMono)
		case "fontSize":
			num(&o.FontSize)
		case "idPrefix":
			str(&o.IDPrefix)
		case "responsive":
			boolean(&o.Responsive)
		case "maxWidth":
			num(&o.MaxWidth)
		case "autoDirection":
			boolean(&o.AutoDirection)
		case "directionBreakpoint":
			num(&o.DirectionBreakpoint)
		case "padding":
			num(&o.Padding)
		case "limit":
			num(&o.Limit)
		case "outputLimit":
			num(&o.OutputLimit)
		case "maxEdges":
			num(&o.MaxEdges)
		case "linkSchemes":
			strs(&o.LinkSchemes)
		case "imageSchemes":
			strs(&o.ImageSchemes)
		case "links":
			boolean(&o.Links)
		case "images":
			boolean(&o.Images)
		case "linkRel":
			str(&o.LinkRel)
		case "now":
			num(&o.Now)
		case "icons":
			var icons map[string]*string
			if ok = json.Unmarshal(value, &icons) == nil; ok {
				o.Icons = func(name string) (string, bool) {
					if s := icons[name]; s != nil {
						return *s, true
					}
					return "", false
				}
			}
		case "config":
			v, err := core.ParseJSON(string(value))
			obj, isObj := v.(*core.Object)
			if ok = err == nil && isObj; ok {
				o.Config = restore(obj).(*core.Object)
			}
		default:
			ok = false
		}
		if !ok {
			return nil, false
		}
	}
	return o, true
}

// Outcome is how one record compared.
type Outcome int

const (
	Pass Outcome = iota
	Fail
	Skip
)

// Check renders one record and compares it with what was recorded.
func Check(reg engine.Registry, rec *Record) (Outcome, string) {
	if rec.Lone {
		return Skip, "lone surrogate"
	}
	if rec.Error != nil && rec.Error.Message == "Diagram is nested too deeply." {
		// The TypeScript library runs out of stack where Go does not.
		return Skip, "nested too deeply"
	}
	opts, ok := Options(rec.Options)
	if !ok {
		return Skip, "options"
	}
	res, err := engine.RenderText(reg, rec.Text, opts)
	if ie, ok := err.(*engine.InternalError); ok {
		return Fail, fmt.Sprintf("internal error: %v\n%s", ie.Value, ie.Stack)
	}
	if rec.Error != nil {
		pe, ok := err.(*core.Error)
		if !ok {
			if err == nil {
				return Fail, fmt.Sprintf("want error %q, got an SVG of %d bytes", rec.Error.Message, len(res.SVG))
			}
			return Fail, fmt.Sprintf("want error %q, got %v", rec.Error.Message, err)
		}
		want := rec.Error
		var diffs []string
		if pe.Message != want.Message {
			diffs = append(diffs, fmt.Sprintf("message:\n  got  %q\n  want %q", pe.Message, want.Message))
		}
		if string(pe.Code) != want.Code {
			diffs = append(diffs, fmt.Sprintf("code: got %s, want %s", pe.Code, want.Code))
		}
		if wt := deref(want.Type); pe.Type != wt {
			diffs = append(diffs, fmt.Sprintf("type: got %q, want %q", pe.Type, wt))
		}
		if wm := deref(want.MermaidType); pe.MermaidType != wm {
			diffs = append(diffs, fmt.Sprintf("mermaidType: got %q, want %q", pe.MermaidType, wm))
		}
		if pe.Line != want.Line || pe.Column != want.Column {
			diffs = append(diffs, fmt.Sprintf("position: got %d:%d, want %d:%d", pe.Line, pe.Column, want.Line, want.Column))
		}
		if pe.Snippet != want.Snippet {
			diffs = append(diffs, fmt.Sprintf("snippet:\n  got  %q\n  want %q", pe.Snippet, want.Snippet))
		}
		if len(diffs) > 0 {
			return Fail, strings.Join(diffs, "\n")
		}
		return Pass, ""
	}
	if rec.Result == nil {
		return Skip, "no result"
	}
	if err != nil {
		return Fail, fmt.Sprintf("want an SVG, got error %v", err)
	}
	want := rec.Result
	var diffs []string
	if res.Type != want.Type || res.MermaidType != want.MermaidType {
		diffs = append(diffs, fmt.Sprintf("type: got %s/%s, want %s/%s", res.Type, res.MermaidType, want.Type, want.MermaidType))
	}
	if res.SVG != want.SVG {
		diffs = append(diffs, "svg: "+FirstDiff(res.SVG, want.SVG))
	}
	if !sameNumber(res.Width, float64(want.Width)) || !sameNumber(res.Height, float64(want.Height)) {
		diffs = append(diffs, fmt.Sprintf("size: got %vx%v, want %vx%v", res.Width, res.Height, want.Width, want.Height))
	}
	if len(res.Links) != len(want.Links) {
		diffs = append(diffs, fmt.Sprintf("links: got %d, want %d", len(res.Links), len(want.Links)))
	} else {
		for i, l := range want.Links {
			g := res.Links[i]
			if g.ID != l.ID || g.Href != l.Href || g.Internal != l.Internal {
				diffs = append(diffs, fmt.Sprintf("link %d: got %+v, want %+v", i, g, l))
			}
		}
	}
	if len(diffs) > 0 {
		return Fail, strings.Join(diffs, "\n")
	}
	return Pass, ""
}

func sameNumber(a, b float64) bool { return a == b || (a != a && b != b) }

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// FirstDiff describes where two strings first differ, with some context.
func FirstDiff(got, want string) string {
	i := 0
	for i < len(got) && i < len(want) && got[i] == want[i] {
		i++
	}
	from := i - 120
	if from < 0 {
		from = 0
	}
	cut := func(s string) string {
		to := i + 160
		if to > len(s) {
			to = len(s)
		}
		if from > len(s) {
			return ""
		}
		return s[from:to]
	}
	return fmt.Sprintf("first difference at byte %d of %d/%d\n  got  …%s…\n  want …%s…", i, len(got), len(want), cut(got), cut(want))
}

// Run checks every record of a diagram type. PELE_GOLDEN_SHOW sets how many failures are shown
// in full (default 5), and PELE_GOLDEN_ONLY runs a single record by its index.
func Run(t *testing.T, typ string, reg engine.Registry) {
	t.Helper()
	records, path, err := Load(typ)
	if err != nil {
		t.Skip(err)
	}
	show := 5
	if s := os.Getenv("PELE_GOLDEN_SHOW"); s != "" {
		show, _ = strconv.Atoi(s)
	}
	only := -1
	if s := os.Getenv("PELE_GOLDEN_ONLY"); s != "" {
		only, _ = strconv.Atoi(s)
	}
	var pass, fail, skip int
	for i := range records {
		if only >= 0 && i != only {
			continue
		}
		outcome, detail := Check(reg, &records[i])
		switch outcome {
		case Pass:
			pass++
		case Skip:
			skip++
		case Fail:
			fail++
			if fail <= show || only >= 0 {
				text := records[i].Text
				if len(text) > 600 && only < 0 {
					text = text[:600] + "…"
				}
				opts, _ := json.Marshal(records[i].Options)
				if len(opts) > 300 && only < 0 {
					opts = append(opts[:300], "…"...)
				}
				t.Errorf("record %d of %s\ntext: %q\noptions: %s\n%s", i, filepath.Base(path), text, opts, detail)
			}
		}
	}
	t.Logf("%s: %d passed, %d failed, %d skipped of %d", filepath.Base(path), pass, fail, skip, len(records))
}
