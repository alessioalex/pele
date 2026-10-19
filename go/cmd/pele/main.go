// Command pele renders a Mermaid diagram to SVG.
//
//	pele [flags] [file]
//
// It reads the diagram from file, or from standard input when no file is given, and writes the
// SVG to standard output or to the file given with -o.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	pele "github.com/alessioalex/pele/go"
)

func main() {
	var (
		out        = flag.String("o", "", "write the SVG to this file instead of standard output")
		maxWidth   = flag.Float64("max-width", 0, "the width the diagram has, in pixels; a type that can draw itself narrower does")
		fontFamily = flag.String("font-family", "", "the font labels are measured and drawn in")
		fontSize   = flag.Float64("font-size", 0, "the base font size in pixels (default 16)")
		idPrefix   = flag.String("id-prefix", "", `prefix for the ids in the SVG (default "pele")`)
		fixed      = flag.Bool("fixed", false, "give the SVG a fixed size instead of scaling it to its container")
		asJSON     = flag.Bool("json", false, "write a JSON object with the SVG, its size, type and links")
	)
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "usage: pele [flags] [file]\n\nRenders a Mermaid diagram to SVG.\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	var src []byte
	var err error
	switch flag.NArg() {
	case 0:
		src, err = io.ReadAll(os.Stdin)
	case 1:
		src, err = os.ReadFile(flag.Arg(0))
	default:
		flag.Usage()
		os.Exit(2)
	}
	if err != nil {
		fail(err)
	}

	opts := &pele.Options{MaxWidth: *maxWidth, FontFamily: *fontFamily, FontSize: *fontSize, IDPrefix: *idPrefix}
	if *fixed {
		opts.Responsive = pele.Bool(false)
	}
	res, err := pele.Render(string(src), opts)
	if err != nil {
		var pe *pele.Error
		if errors.As(err, &pe) && pe.Line > 0 {
			fmt.Fprintf(os.Stderr, "pele: %s\n", pe.Message)
			os.Exit(1)
		}
		fail(err)
	}

	body := []byte(res.SVG)
	if *asJSON {
		body, err = json.MarshalIndent(res, "", "  ")
		if err != nil {
			fail(err)
		}
	}
	body = append(body, '\n')
	if *out != "" {
		err = os.WriteFile(*out, body, 0o644)
	} else {
		_, err = os.Stdout.Write(body)
	}
	if err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "pele: %v\n", err)
	os.Exit(1)
}
